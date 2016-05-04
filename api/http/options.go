package http

func (ctx *context) SetMaxConnsPerHost(conns int) {
	ctx.client.MaxConnsPerHost = conns
}

func (ctx *context) SetDefaults(args RequestArgs) {
	ctx.defaults = args
}
