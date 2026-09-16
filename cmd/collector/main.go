package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/markusheinemann/downline/packages/config"
	"github.com/markusheinemann/downline/packages/openskynetwork"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

func main() {

	ctx := context.Background()
	cfg, err := loadConfig(os.Args[1:])
	if err != nil {
		log.Fatalf("failed to load config:\n%v", err)
	}

	client := createOpenSkyClient(ctx, cfg.OpenSky)

	res, err := client.ListAllStateVectors(ctx, openskynetwork.StateVectorOptions{})
	if err != nil {
		log.Fatalf("%v", err)
	}

	fmt.Println(res)
}

func createOpenSkyClient(ctx context.Context, cfg config.OpenSky) *openskynetwork.Client {
	conf := clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     cfg.TokenURL,
	}

	ts := conf.TokenSource(ctx)
	tc := oauth2.NewClient(ctx, ts)
	return openskynetwork.NewClient(tc)
}
