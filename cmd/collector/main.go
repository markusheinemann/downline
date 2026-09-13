package main

import (
	"context"
	"fmt"
	"log"

	"github.com/markusheinemann/downline/packages/openskynetwork"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

func main() {

	ctx := context.Background()
	client := createOpenSkyClient(ctx)

	res, err := client.ListAllStateVectors(ctx, openskynetwork.StateVectorOptions{})
	if err != nil {
		log.Fatalf("%v", err)
	}

	fmt.Println(res)
}

func createOpenSkyClient(ctx context.Context) *openskynetwork.Client {
	conf := clientcredentials.Config{
		ClientID:     "XXX",
		ClientSecret: "XXX",
		TokenURL:     "https://auth.opensky-network.org/auth/realms/opensky-network/protocol/openid-connect/token",
	}

	ts := conf.TokenSource(ctx)
	tc := oauth2.NewClient(ctx, ts)
	return openskynetwork.NewClient(tc)
}
