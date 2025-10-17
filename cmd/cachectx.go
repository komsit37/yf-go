package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	yfgo "github.com/komsit37/yf-go"
)

func requestContext(cmd *cobra.Command) context.Context {
	ctx := cmd.Context()
	var opts []yfgo.RequestOption

	if viper.GetBool("no-cache") {
		opts = append(opts, yfgo.BypassCache())
	}
	if ttl := viper.GetDuration("cache-ttl"); ttl > 0 {
		opts = append(opts, yfgo.CacheTTL(ttl))
	}
	if viper.GetBool("force-refresh") {
		opts = append(opts, yfgo.ForceRefresh())
	}
	if len(opts) == 0 {
		return ctx
	}
	return yfgo.WithCacheOptions(ctx, opts...)
}
