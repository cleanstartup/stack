package hugo

func Layouts(baseDir string, includes ...string) Part {
	return partFunc(func(app *WebApp) {
		if app == nil {
			return
		}
		app.RegisterLayouts(moduleRoot(baseDir), includes...)
	})
}
