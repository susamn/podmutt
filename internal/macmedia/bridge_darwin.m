//go:build darwin && cgo

#import <AppKit/AppKit.h>
#import <MediaPlayer/MediaPlayer.h>
#import <dispatch/dispatch.h>
#include "bridge.h"
#include "_cgo_export.h"

@interface PMMedia : NSObject
@property(nonatomic,assign) uintptr_t handle;
@property(atomic,assign) BOOL active;
@property(nonatomic,retain) NSMutableArray *commands;
@property(nonatomic,retain) NSMutableArray *tokens;
@end
@implementation PMMedia
- (void)dealloc { [_commands release]; [_tokens release]; [super dealloc]; }
@end
static void onMain(dispatch_block_t block) {
    if ([NSThread isMainThread]) block(); else dispatch_sync(dispatch_get_main_queue(), block);
}
void pm_prepare(void) {
    @autoreleasepool {
        [NSApplication sharedApplication];
        [NSApp setActivationPolicy:NSApplicationActivationPolicyProhibited];
        [NSApp finishLaunching];
    }
}
void pm_run(void) { @autoreleasepool { [NSApp run]; } }
void pm_stop(void) {
    dispatch_async(dispatch_get_main_queue(), ^{
        [NSApp stop:nil];
        // stop: alone does not wake a blocked nextEventMatchingMask call.
        NSEvent *event = [NSEvent otherEventWithType:NSEventTypeApplicationDefined
            location:NSZeroPoint modifierFlags:0 timestamp:0 windowNumber:0
            context:nil subtype:0 data1:0 data2:0];
        [NSApp postEvent:event atStart:YES];
    });
}
void *pm_install(uintptr_t handle) {
    __block PMMedia *controller = nil;
    onMain(^{ @autoreleasepool {
        if ([[[NSBundle mainBundle] bundleIdentifier] length] == 0) return;
        controller = [[PMMedia alloc] init];
        controller.handle = handle; controller.active = YES;
        controller.commands = [NSMutableArray array]; controller.tokens = [NSMutableArray array];
        MPRemoteCommandCenter *center = [MPRemoteCommandCenter sharedCommandCenter];
        NSArray *commands = @[center.playCommand, center.pauseCommand,
            center.togglePlayPauseCommand, center.stopCommand, center.nextTrackCommand];
        for (NSUInteger i = 0; i < [commands count]; i++) {
            MPRemoteCommand *command = commands[i];
            PMMedia *owner = controller; // copied handler retains owner even for a late delivery
            id token = [command addTargetWithHandler:^MPRemoteCommandHandlerStatus(MPRemoteCommandEvent *event) {
                (void)event;
                if (!owner.active) return MPRemoteCommandHandlerStatusCommandFailed;
                return podmuttMediaEvent(owner.handle, (int)i)
                    ? MPRemoteCommandHandlerStatusSuccess : MPRemoteCommandHandlerStatusCommandFailed;
            }];
            [controller.commands addObject:command]; [controller.tokens addObject:token];
            command.enabled = NO;
        }
        center.previousTrackCommand.enabled = NO;
        center.changePlaybackPositionCommand.enabled = NO;
        center.seekForwardCommand.enabled = NO; center.seekBackwardCommand.enabled = NO;
        center.skipForwardCommand.enabled = NO; center.skipBackwardCommand.enabled = NO;
        center.changePlaybackRateCommand.enabled = NO;
    }});
    return controller;
}
void pm_update(void *pointer, const char *title, double position, double duration, int state, int current, int next) {
    if (!pointer) return;
    onMain(^{ @autoreleasepool {
        PMMedia *controller = (PMMedia *)pointer;
        if (!controller.active) return;
        MPNowPlayingInfoCenter *info = [MPNowPlayingInfoCenter defaultCenter];
        if (current) {
            info.nowPlayingInfo = @{MPMediaItemPropertyTitle: [NSString stringWithUTF8String:title] ?: @"podmutt",
                MPMediaItemPropertyPlaybackDuration: @(duration),
                MPNowPlayingInfoPropertyElapsedPlaybackTime: @(position),
                MPNowPlayingInfoPropertyPlaybackRate: @(state == 1 ? 1.0 : 0.0)};
        } else { info.nowPlayingInfo = nil; }
        info.playbackState = state == 1 ? MPNowPlayingPlaybackStatePlaying
            : state == 2 ? MPNowPlayingPlaybackStatePaused : MPNowPlayingPlaybackStateStopped;
        MPRemoteCommandCenter *center = [MPRemoteCommandCenter sharedCommandCenter];
        center.playCommand.enabled = current && state != 1;
        center.pauseCommand.enabled = state == 1;
        center.togglePlayPauseCommand.enabled = current;
        center.stopCommand.enabled = current;
        center.nextTrackCommand.enabled = next;
    }});
}
void pm_close(void *pointer) {
    if (!pointer) return;
    onMain(^{ @autoreleasepool {
        PMMedia *controller = (PMMedia *)pointer;
        controller.active = NO;
        for (NSUInteger i = 0; i < [controller.commands count]; i++) {
            MPRemoteCommand *command = controller.commands[i];
            command.enabled = NO; [command removeTarget:controller.tokens[i]];
        }
        [controller.tokens removeAllObjects]; [controller.commands removeAllObjects];
        MPNowPlayingInfoCenter *info = [MPNowPlayingInfoCenter defaultCenter];
        info.playbackState = MPNowPlayingPlaybackStateStopped; info.nowPlayingInfo = nil;
        [controller release];
    }});
}
