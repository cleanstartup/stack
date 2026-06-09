package hugo

func Content(baseDir string, includes ...string) Part {
	return partFunc(func(app *WebApp) {
		if app == nil {
			return
		}
		app.RegisterContent(moduleRoot(baseDir), includes...)
	})
}
