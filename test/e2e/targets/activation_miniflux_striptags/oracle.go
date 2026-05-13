package activation_miniflux_striptags

const (
	directInvocationInput    = `<p>Hello <strong>reader</strong></p><script>alert(1)</script>`
	directInvocationExpected = "Hello readeralert(1)"
)
