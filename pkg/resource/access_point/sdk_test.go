// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

package access_point

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/s3control"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	ackmetrics "github.com/aws-controllers-k8s/runtime/pkg/metrics"
	svcapitypes "github.com/aws-controllers-k8s/s3control-controller/apis/v1alpha1"
)

const (
	testAccountID = "012345678901"
	testAPName    = "ap-outpost"
)

// capturedRequest records the shape of an HTTP request the SDK emitted, so the
// test can assert not merely THAT a call happened but that the right operation
// was invoked with the right parameters.
type capturedRequest struct {
	method    string
	path      string
	accountID string // value of the X-Amz-Account-Id header
}

// routingHTTPClient is a mock aws.HTTPClient that identifies each SDK request
// by its wire shape (method + URL path + account header), records it, and
// returns a canned XML body per operation. Unlike a bare call-counter, it lets
// the test prove GetAccessPointPolicy was called with the expected shape.
//
// s3control operations (restxml, v20180820):
//
//	GetAccessPoint        GET /v20180820/accesspoint/{Name}
//	GetAccessPointPolicy  GET /v20180820/accesspoint/{Name}/policy
//
// Both carry the X-Amz-Account-Id header. The policy op is distinguished by its
// path ending in "/policy".
type routingHTTPClient struct {
	requests []capturedRequest

	getAccessPointBody string
	getPolicyBody      string
}

func (c *routingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	cap := capturedRequest{
		method:    req.Method,
		path:      req.URL.Path,
		accountID: req.Header.Get("X-Amz-Account-Id"),
	}
	c.requests = append(c.requests, cap)

	body := c.getAccessPointBody
	if strings.HasSuffix(req.URL.Path, "/policy") {
		body = c.getPolicyBody
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Status:     http.StatusText(http.StatusOK),
		Header:     http.Header{"Content-Type": []string{"application/xml"}},
		Body:       io.NopCloser(strings.NewReader(body)),
	}, nil
}

// policyRequest returns the recorded GetAccessPointPolicy request (path ending
// in "/policy"), or nil if the SDK never made one.
func (c *routingHTTPClient) policyRequest() *capturedRequest {
	for i := range c.requests {
		if strings.HasSuffix(c.requests[i].path, "/policy") {
			return &c.requests[i]
		}
	}
	return nil
}

// TestSdkFind_PolicyFetchedWhenARNAbsent is the regression guard for the
// S3-on-Outposts fix. GetAccessPoint returns a response with NO
// AccessPointArn element -- exactly what the service does for Outposts access
// points -- and sdkFind must STILL issue a GetAccessPointPolicy request. The
// removed `ARN != nil` guard used to skip the policy read in precisely this
// case.
func TestSdkFind_PolicyFetchedWhenARNAbsent(t *testing.T) {
	require := require.New(t)
	assert := assert.New(t)

	httpc := &routingHTTPClient{
		// GetAccessPoint response: name/bucket present, <AccessPointArn> OMITTED
		// (the Outposts shape).
		getAccessPointBody: `<?xml version="1.0" encoding="UTF-8"?>
<GetAccessPointResult>
  <Name>` + testAPName + `</Name>
  <Bucket>test-bucket</Bucket>
  <NetworkOrigin>VPC</NetworkOrigin>
</GetAccessPointResult>`,
		getPolicyBody: `<?xml version="1.0" encoding="UTF-8"?>
<GetAccessPointPolicyResult>
  <Policy>{"Version":"2012-10-17"}</Policy>
</GetAccessPointPolicyResult>`,
	}

	client := svcsdk.New(svcsdk.Options{
		HTTPClient: httpc,
		Region:     "us-west-2",
	})
	rm := &resourceManager{
		sdkapi:  client,
		metrics: ackmetrics.NewMetrics("s3control"),
	}

	r := &resource{ko: &svcapitypes.AccessPoint{}}
	r.ko.Spec.AccountID = aws.String(testAccountID)
	r.ko.Spec.Name = aws.String(testAPName)

	latest, err := rm.sdkFind(context.Background(), r)

	require.NoError(err)
	require.NotNil(latest)

	// --- The assertion that encodes the fix: a GetAccessPointPolicy request was
	// issued, AND it had the expected shape (right op, right name, right account).
	polReq := httpc.policyRequest()
	require.NotNil(polReq,
		"sdkFind MUST issue GetAccessPointPolicy even when AccessPointArn is nil (Outposts); "+
			"the removed ARN guard used to skip it")
	assert.Equal("GET", polReq.method)
	assert.True(strings.HasSuffix(polReq.path, "/accesspoint/"+testAPName+"/policy"),
		"policy request path should target this access point's /policy subresource, got %q", polReq.path)
	assert.Equal(testAccountID, polReq.accountID,
		"policy request should carry the resource's account id in X-Amz-Account-Id")

	// The fetched policy landed on the observed resource.
	require.NotNil(latest.ko.Spec.Policy)
	assert.Equal(`{"Version":"2012-10-17"}`, *latest.ko.Spec.Policy)

	// Sanity: the read genuinely had no ARN
	if latest.ko.Status.ACKResourceMetadata != nil {
		assert.Nil(latest.ko.Status.ACKResourceMetadata.ARN)
	}
}
