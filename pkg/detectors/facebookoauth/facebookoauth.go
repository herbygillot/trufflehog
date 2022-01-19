package facebookoauth

import (
	"context"
	"fmt"

	// "log"
	"regexp"
	"strings"

	"net/http"

	"github.com/trufflesecurity/trufflehog/pkg/common"
	"github.com/trufflesecurity/trufflehog/pkg/detectors"
	"github.com/trufflesecurity/trufflehog/pkg/pb/detectorspb"
)

type Scanner struct{}

// Ensure the Scanner satisfies the interface at compile time
var _ detectors.Detector = (*Scanner)(nil)

var (
	client = common.SaneHttpClient()

	//Make sure that your group is surrounded in boundry characters such as below to reduce false positives
	apiIdPat     = regexp.MustCompile(detectors.PrefixRegex([]string{"facebook"}) + `\b([0-9]{15,18})\b`) // not actually sure of the upper bound
	apiSecretPat = regexp.MustCompile(detectors.PrefixRegex([]string{"facebook"}) + `\b([A-Za-z0-9]{32})\b`)
)

// Keywords are used for efficiently pre-filtering chunks.
// Use identifiers in the secret preferably, or the provider name.
func (s Scanner) Keywords() []string {
	return []string{"facebook"}
}

// FromData will find and optionally verify FacebookOAuth secrets in a given set of bytes.
func (s Scanner) FromData(ctx context.Context, verify bool, data []byte) (results []detectors.Result, err error) {
	dataStr := string(data)

	apiIdMatches := apiIdPat.FindAllStringSubmatch(dataStr, -1)
	apiSecretMatches := apiSecretPat.FindAllStringSubmatch(dataStr, -1)

	for _, apiIdMatch := range apiIdMatches {
		if len(apiIdMatch) != 2 {
			continue
		}
		apiIdRes := strings.TrimSpace(apiIdMatch[1])

		for _, apiSecretMatch := range apiSecretMatches {
			if len(apiSecretMatch) != 2 {
				continue
			}
			apiSecretRes := strings.TrimSpace(apiSecretMatch[1])

			s1 := detectors.Result{
				DetectorType: detectorspb.DetectorType_FacebookOAuth,
				Redacted:     apiIdRes,
				Raw:          []byte(apiSecretRes),
			}

			if verify {
				// thanks https://stackoverflow.com/questions/15621471/validate-a-facebook-app-id-and-app-secret
				req, _ := http.NewRequest("GET", fmt.Sprintf("https://graph.facebook.com/%s?fields=roles&access_token=%s|%s", apiIdRes, apiIdRes, apiSecretRes), nil)
				res, err := client.Do(req)
				if err == nil {
					defer res.Body.Close()
					if res.StatusCode >= 200 && res.StatusCode < 300 {
						s1.Verified = true
					} else {
						if detectors.IsKnownFalsePositive(apiIdRes, detectors.DefaultFalsePositives, true) {
							continue
						}
					}
				}
			}

			results = append(results, s1)
		}
	}

	return detectors.CleanResults(results), nil
}