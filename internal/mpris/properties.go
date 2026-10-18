package mpris

import "github.com/godbus/dbus/v5"

type properties struct{ s *Service }

func (p *properties) Get(iface, name string) (dbus.Variant, *dbus.Error) {
	all, err := p.GetAll(iface)
	if err != nil {
		return dbus.Variant{}, err
	}
	value, ok := all[name]
	if !ok {
		return dbus.Variant{}, busError("UnknownProperty")
	}
	return value, nil
}
func (p *properties) GetAll(iface string) (map[string]dbus.Variant, *dbus.Error) {
	if iface == rootInterface {
		return rootValues(), nil
	}
	if iface != playerInterface {
		return nil, busError("UnknownInterface")
	}
	p.s.mu.Lock()
	defer p.s.mu.Unlock()
	result := make(values, len(p.s.values))
	for key, value := range p.s.values {
		result[key] = value
	}
	return result, nil
}
func (p *properties) Set(iface, name string, value dbus.Variant) *dbus.Error {
	if _, err := p.Get(iface, name); err != nil {
		return err
	}
	if iface != playerInterface {
		return busError("PropertyReadOnly")
	}
	switch name {
	case "Rate", "Volume":
		number, ok := value.Value().(float64)
		if !ok {
			return busError("InvalidArgs")
		}
		if name == "Rate" && number == 0 {
			return p.s.act("pause")
		}
		if number == 1 {
			return nil
		}
		return busError("NotSupported")
	case "LoopStatus":
		text, ok := value.Value().(string)
		if !ok {
			return busError("InvalidArgs")
		}
		if text == "None" {
			return nil
		}
		return busError("NotSupported")
	case "Shuffle":
		enabled, ok := value.Value().(bool)
		if !ok {
			return busError("InvalidArgs")
		}
		if !enabled {
			return nil
		}
		return busError("NotSupported")
	}
	return busError("PropertyReadOnly")
}
func busError(kind string) *dbus.Error {
	return dbus.NewError("org.freedesktop.DBus.Error."+kind, []any{"podmutt: " + kind})
}
