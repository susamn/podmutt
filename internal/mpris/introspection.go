package mpris

import (
	"sort"

	"github.com/godbus/dbus/v5/introspect"
)

func (s *Service) export() error {
	root, player, props := &rootMethods{}, &playerMethods{s}, &properties{s}
	node := &introspect.Node{Name: string(Path), Interfaces: []introspect.Interface{
		{Name: rootInterface, Methods: introspect.Methods(root), Properties: describe(rootValues(), true)},
		{Name: playerInterface, Methods: append(introspect.Methods(player), introspect.Method{Name: "Seek", Args: []introspect.Arg{{Name: "Offset", Type: "x", Direction: "in"}}}), Properties: describe(s.values, false),
			Signals: []introspect.Signal{{Name: "Seeked", Args: []introspect.Arg{{Name: "Position", Type: "x"}}}}},
		{Name: propertiesInterface, Methods: introspect.Methods(props),
			Signals: []introspect.Signal{{Name: "PropertiesChanged", Args: []introspect.Arg{
				{Name: "Interface", Type: "s"}, {Name: "Changed", Type: "a{sv}"}, {Name: "Invalidated", Type: "as"},
			}}}},
	}}
	for _, binding := range []struct {
		value any
		iface string
	}{
		{root, rootInterface}, {props, propertiesInterface},
		{introspect.NewIntrospectable(node), "org.freedesktop.DBus.Introspectable"},
	} {
		if err := s.conn.Export(binding.value, Path, binding.iface); err != nil {
			return err
		}
	}
	return s.conn.ExportMethodTable(player.table(), Path, playerInterface)
}
func describe(props values, root bool) []introspect.Property {
	result := make([]introspect.Property, 0, len(props))
	for name, value := range props {
		access, emits := "read", "true"
		if root || name == "CanControl" {
			emits = "const"
		}
		if name == "Position" {
			emits = "false"
		}
		switch name {
		case "Rate", "Volume", "LoopStatus", "Shuffle":
			access = "readwrite"
		}
		result = append(result, introspect.Property{Name: name, Type: value.Signature().String(), Access: access,
			Annotations: []introspect.Annotation{{Name: propertiesInterface + ".EmitsChangedSignal", Value: emits}}})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}
