package exporter

import (
	"encoding/json"
	"fmt"
	"strconv"
    "strings"

	log "github.com/sirupsen/logrus"
)

// gatherData - Collects the data from the API and stores into struct
func (e *Exporter) gatherData() ([]*Datum, *RateLimits, error) {

	data := []*Datum{}

	responses, err := asyncHTTPGets(e.TargetURLs, e.APIToken)

	if err != nil {
		return data, nil, err
	}

	for _, response := range responses {

		// Github can at times present an array, or an object for the same data set.
		// This code checks handles this variation.
		if isArray(response.body) {
			ds := []*Datum{}
			json.Unmarshal(response.body, &ds)
			data = append(data, ds...)
		} else {
			d := new(Datum)

			// Get releases
			if strings.Contains(response.url, "/repos/") {
				getReleases(e, response.url, &d.Releases)
			}
			json.Unmarshal(response.body, &d)
			data = append(data, d)
		}

		log.Infof("API data fetched for repository: %s", response.url)
	}

	// Check the API rate data and store as a metric
	rates, err := getRates(e.APIURL, e.APIToken)

	if err != nil {
		log.Errorf("Unable to obtain rate limit data from API, Error: %s", err)
	}


	//return data, rates, err
	return data, rates, nil

}

// getRates obtains the rate limit data for requests against the github API.
// Especially useful when operating without oauth and the subsequent lower cap.
func getRates(baseURL string, token string) (*RateLimits, error) {

	rateEndPoint := ("/rate_limit")
	url := fmt.Sprintf("%s%s", baseURL, rateEndPoint)

	resp, err := getHTTPResponse(url, token)
	if err != nil {
		return &RateLimits{}, err
	}
	defer resp.Body.Close()

	// Triggers if rate-limiting isn't enabled on private Github Enterprise installations
	if resp.StatusCode == 404 {
		return &RateLimits{}, fmt.Errorf("Rate Limiting not enabled in GitHub API")
	}

	limit, err := strconv.ParseFloat(resp.Header.Get("X-RateLimit-Limit"), 64)

	if err != nil {
		return &RateLimits{}, err
	}

	rem, err := strconv.ParseFloat(resp.Header.Get("X-RateLimit-Remaining"), 64)

	if err != nil {
		return &RateLimits{}, err
	}

	reset, err := strconv.ParseFloat(resp.Header.Get("X-RateLimit-Reset"), 64)

	if err != nil {
		return &RateLimits{}, err
	}

	return &RateLimits{
		Limit:     limit,
		Remaining: rem,
		Reset:     reset,
	}, err

}

func getReleases(e *Exporter, url string, data *[]Release) {
	i := strings.Index(url, "?")
	baseURL := url[:i]
	releasesURL := baseURL + "/releases"
	releasesResponse, err := asyncHTTPGets([]string{releasesURL}, e.APIToken)

	if err != nil {
		log.Errorf("Unable to obtain releases from API, Error: %s", err)
	}

	json.Unmarshal(releasesResponse[0].body, &data)
}

// isArray simply looks for key details that determine if the JSON response is an array or not.
func isArray(body []byte) bool {

	isArray := false

	for _, c := range body {
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			continue
		}
		isArray = c == '['
		break
	}

	return isArray

}
