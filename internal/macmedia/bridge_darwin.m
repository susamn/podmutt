//go:build darwin && cgo

#import <AppKit/AppKit.h>
#import <MediaPlayer/MediaPlayer.h>
#import <dispatch/dispatch.h>
#include <stdlib.h>
#include <string.h>
#include "bridge.h"
#include "_cgo_export.h"

typedef MPRemoteCommandHandlerStatus (^PMHandler)(MPRemoteCommandEvent *);
@interface PMMedia : NSObject
@property(nonatomic,assign) uintptr_t handle;
@property(atomic,assign) BOOL active;
@property(nonatomic,retain) NSMutableArray *commands;
@property(nonatomic,retain) NSMutableArray *tokens;
#ifdef PODMUTT_NATIVE_CHECK
@property(nonatomic,retain) NSMutableArray *handlers;
#endif
@end
@implementation PMMedia
- (void)dealloc {
    [_commands release]; [_tokens release];
#ifdef PODMUTT_NATIVE_CHECK
    [_handlers release];
#endif
    [super dealloc];
}
@end
static void onMain(dispatch_block_t block) {
    if ([NSThread isMainThread]) block(); else dispatch_sync(dispatch_get_main_queue(), block);
}
int pm_prepare(void) {
    // A caller on another Go thread or an unbundled developer binary must fall
    // back nonfatally, not initialize AppKit on the wrong thread or seize keys.
    if (![NSThread isMainThread]) return 0;
    @autoreleasepool { @try {
        if ([[[NSBundle mainBundle] bundleIdentifier] length] == 0) return 0;
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyProhibited];
        [NSApp finishLaunching];
        return 1;
    } @catch (NSException *exception) { (void)exception; return 0; } }
}
void pm_run(void) { @autoreleasepool { [NSApp run]; } }
void pm_stop(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [NSApp stop:nil];
        NSEvent *event = [NSEvent otherEventWithType:NSEventTypeApplicationDefined
            location:NSZeroPoint modifierFlags:0 timestamp:0 windowNumber:0
            context:nil subtype:0 data1:0 data2:0];
        [NSApp postEvent:event atStart:YES];
    });
}
// Main-thread-only teardown, including partially installed handlers. Continue
// unregistering if an individual framework operation fails; never log metadata.
static void discard(PMMedia *controller) {
    controller.active = NO;
    for (NSUInteger i = 0; i < [controller.commands count]; i++) {
        @try {
            MPRemoteCommand *command = controller.commands[i];
            command.enabled = NO; [command removeTarget:controller.tokens[i]];
        } @catch (NSException *exception) { (void)exception; }
    }
    [controller.tokens removeAllObjects]; [controller.commands removeAllObjects];
#ifdef PODMUTT_NATIVE_CHECK
    [controller.handlers removeAllObjects];
#endif
    @try {
        MPNowPlayingInfoCenter *info = [MPNowPlayingInfoCenter defaultCenter];
        info.playbackState = MPNowPlayingPlaybackStateStopped; info.nowPlayingInfo = nil;
    } @catch (NSException *exception) { (void)exception; }
    [controller release];
}
void *pm_install(uintptr_t handle) {
    __block PMMedia *controller = nil;
    onMain(^{ @autoreleasepool { @try {
        if ([[[NSBundle mainBundle] bundleIdentifier] length] == 0) return;
        controller = [[PMMedia alloc] init];
        controller.handle = handle; controller.active = YES;
        controller.commands = [NSMutableArray array]; controller.tokens = [NSMutableArray array];
#ifdef PODMUTT_NATIVE_CHECK
        controller.handlers = [NSMutableArray array];
#endif
        MPRemoteCommandCenter *center = [MPRemoteCommandCenter sharedCommandCenter];
        NSArray *commands = @[center.playCommand, center.pauseCommand,
            center.togglePlayPauseCommand, center.stopCommand, center.nextTrackCommand,
            center.skipForwardCommand, center.skipBackwardCommand];
        center.skipForwardCommand.preferredIntervals = @[@15];
        center.skipBackwardCommand.preferredIntervals = @[@15];
        for (NSUInteger i = 0; i < [commands count]; i++) {
            MPRemoteCommand *command = commands[i];
            PMMedia *owner = controller; // copied handler retains a late delivery's owner
            PMHandler handler = ^MPRemoteCommandHandlerStatus(MPRemoteCommandEvent *event) {
                (void)event;
                if (!owner.active) return MPRemoteCommandHandlerStatusCommandFailed;
                return podmuttMediaEvent(owner.handle, (int)i)
                    ? MPRemoteCommandHandlerStatusSuccess : MPRemoteCommandHandlerStatusCommandFailed;
            };
            id token = [command addTargetWithHandler:handler];
            if (!token) [NSException raise:@"PMRegistrationFailure" format:@"Remote target unavailable"];
            [controller.tokens addObject:token]; [controller.commands addObject:command];
#ifdef PODMUTT_NATIVE_CHECK
            [controller.handlers addObject:[[handler copy] autorelease]];
#endif
            command.enabled = NO;
        }
        center.previousTrackCommand.enabled = NO;
        center.changePlaybackPositionCommand.enabled = NO;
        center.seekForwardCommand.enabled = NO; center.seekBackwardCommand.enabled = NO;
        center.skipForwardCommand.enabled = NO; center.skipBackwardCommand.enabled = NO;
        center.changePlaybackRateCommand.enabled = NO;
        center.changeRepeatModeCommand.enabled = NO; center.changeShuffleModeCommand.enabled = NO;
    } @catch (NSException *exception) {
        (void)exception; if (controller) discard(controller); controller = nil;
    } }});
    return controller;
}
int pm_update(void *pointer, const char *title, double position, double duration, int state, int current, int next, int seekable) {
    if (!pointer) return 0;
    __block int success = 0;
    onMain(^{ @autoreleasepool { @try {
        PMMedia *controller = (PMMedia *)pointer;
        if (!controller.active) return;
        // Publish capabilities before metadata/state so the system observes the
        // new queue availability with this update, not the previous command mask.
        MPRemoteCommandCenter *center = [MPRemoteCommandCenter sharedCommandCenter];
        center.playCommand.enabled = current && state != 1;
        center.pauseCommand.enabled = state == 1;
        center.togglePlayPauseCommand.enabled = current;
        center.stopCommand.enabled = current;
        center.nextTrackCommand.enabled = next;
        center.skipForwardCommand.enabled = seekable;
        center.skipBackwardCommand.enabled = seekable;
        MPNowPlayingInfoCenter *info = [MPNowPlayingInfoCenter defaultCenter];
        if (current) {
            NSMutableDictionary *metadata = [NSMutableDictionary dictionaryWithDictionary:@{
                MPMediaItemPropertyTitle: [NSString stringWithUTF8String:title] ?: @"podmutt",
                MPNowPlayingInfoPropertyElapsedPlaybackTime: @(position),
                MPNowPlayingInfoPropertyPlaybackRate: @(state == 1 ? 1.0 : 0.0)}];
            if (duration > 0) metadata[MPMediaItemPropertyPlaybackDuration] = @(duration);
            info.nowPlayingInfo = metadata;
        } else { info.nowPlayingInfo = nil; }
        info.playbackState = state == 1 ? MPNowPlayingPlaybackStatePlaying
            : state == 2 ? MPNowPlayingPlaybackStatePaused : MPNowPlayingPlaybackStateStopped;
        success = 1;
    } @catch (NSException *exception) { (void)exception; } }});
    return success;
}
void pm_close(void *pointer) {
    if (pointer) onMain(^{ @autoreleasepool { discard((PMMedia *)pointer); } });
}
#ifdef PODMUTT_NATIVE_CHECK
// Native diagnostic hooks are excluded from ordinary application builds. The
// captured block is the SAME block registered with MediaPlayer, not a Go shortcut.
void *pm_check_capture(void *pointer, int action) {
    __block id handler = nil;
    onMain(^{ PMMedia *owner = pointer;
        if (owner && action >= 0 && (NSUInteger)action < [owner.handlers count])
            handler = [owner.handlers[action] copy];
    });
    return handler;
}
int pm_check_fire(void *pointer) { return pointer && ((PMHandler)pointer)(nil) == MPRemoteCommandHandlerStatusSuccess; }
void pm_check_release(void *pointer) { [(id)pointer release]; }
int pm_check_controls(void) {
    __block int mask = 0;
    onMain(^{ MPRemoteCommandCenter *c = [MPRemoteCommandCenter sharedCommandCenter];
        NSArray *commands = @[c.playCommand,c.pauseCommand,c.togglePlayPauseCommand,c.stopCommand,c.nextTrackCommand,
            c.previousTrackCommand,c.changePlaybackPositionCommand,c.seekForwardCommand,c.seekBackwardCommand,
            c.skipForwardCommand,c.skipBackwardCommand,c.changePlaybackRateCommand,
            c.changeRepeatModeCommand,c.changeShuffleModeCommand];
        for (NSUInteger i=0;i<[commands count];i++) if (((MPRemoteCommand *)commands[i]).enabled) mask |= 1 << i;
    }); return mask;
}
int pm_check_state(void) {
    __block int state = 0; onMain(^{ MPNowPlayingPlaybackState s = [MPNowPlayingInfoCenter defaultCenter].playbackState;
        state = s == MPNowPlayingPlaybackStatePlaying ? 1 : s == MPNowPlayingPlaybackStatePaused ? 2 : 0;
    }); return state;
}
char *pm_check_title(void) {
    __block char *title = NULL; onMain(^{ NSString *value = [MPNowPlayingInfoCenter defaultCenter].nowPlayingInfo[MPMediaItemPropertyTitle];
        title = strdup(value ? [value UTF8String] : "");
    }); return title;
}
#endif
