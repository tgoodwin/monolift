package pluginedge

import "plugin"

func Entry(name string) (*plugin.Plugin, error) {
	return plugin.Open(name)
}
