package eventx

import "github.com/arcgolabs/pkg/option"

func buildSubscribeOptions(opts ...SubscribeOption) subscribeOptions {
	cfg := defaultSubscribeOptions()
	option.Apply(&cfg, opts...)
	return cfg
}
