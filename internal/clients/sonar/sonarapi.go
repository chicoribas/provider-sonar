package sonar

import (
	"context"
	"io"
	"log"
	"net/http"
	"net/url"
)

type SonarApiOptions struct {
	Key     string
	BaseUrl string
}

type SonarApi struct {
	Options SonarApiOptions
}

type SonarPaging struct {
	PageIndex int `json:"pageIndex"`
	PageSize  int `json:"pageSize"`
	Total     int `json:"total"`
}

func NewSonarApi(options SonarApiOptions) SonarApi {
	// Default values for Foo.
	opt := SonarApiOptions{
		BaseUrl: "https://sonarcloud.io",
	}

	if options.BaseUrl == "" {
		options.BaseUrl = opt.BaseUrl
	}

	return SonarApi{
		Options: options,
	}
}

func (sonarApi SonarApi) GetUrl(uri string) *url.URL {
	u, err := url.Parse(sonarApi.Options.BaseUrl)
	if err != nil {
		log.Fatal(err)
	}

	return u.JoinPath(uri)
}

func (sonarApi SonarApi) NewRequest(ctx context.Context, method string, url string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	req.SetBasicAuth(sonarApi.Options.Key, "")

	return req, err
}
