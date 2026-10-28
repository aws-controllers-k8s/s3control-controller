// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
//
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

	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	ackmetrics "github.com/aws-controllers-k8s/runtime/pkg/metrics"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/s3control"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	svcapitypes "github.com/aws-controllers-k8s/s3control-controller/apis/v1alpha1"
)

const (
	generalPurposeARN = "arn:aws:s3:us-west-2:012345678901:accesspoint/" + testAPName
	outpostsARN       = "arn:aws:s3-outposts:us-west-2:012345678901:outpost/op-0123456789abcdef0/accesspoint/" + testAPName
	outpostsBucketARN = "arn:aws:s3-outposts:us-west-2:012345678901:outpost/op-0123456789abcdef0/bucket/test-bucket"
)

// taggingHTTPClient answers GetAccessPoint, GetAccessPointPolicy,
// ListTagsForResource and CreateAccessPoint with canned bodies and records
// every request with its body.
type taggingHTTPClient struct {
	requests        []capturedRequest
	bodies          []string
	accessPointBody string
}

func (c *taggingHTTPClient) Do(req *http.Request) (*http.Response, error) {
	body := ""
	if req.Body != nil {
		b, _ := io.ReadAll(req.Body)
		body = string(b)
	}
	c.requests = append(c.requests, capturedRequest{method: req.Method, path: req.URL.Path, accountID: req.Header.Get("X-Amz-Account-Id")})
	c.bodies = append(c.bodies, body)
	resp := c.accessPointBody
	switch {
	case strings.Contains(req.URL.Path, "/tags/"):
		resp = `<?xml version="1.0" encoding="UTF-8"?><ListTagsForResourceResult><Tags><Tag><Key>team</Key><Value>platform</Value></Tag></Tags></ListTagsForResourceResult>`
	case strings.HasSuffix(req.URL.Path, "/policy"):
		resp = `<?xml version="1.0" encoding="UTF-8"?><GetAccessPointPolicyResult><Policy>{"Version":"2012-10-17"}</Policy></GetAccessPointPolicyResult>`
	case req.Method == http.MethodPut:
		resp = `<?xml version="1.0" encoding="UTF-8"?><CreateAccessPointResult><AccessPointArn>` + generalPurposeARN + `</AccessPointArn><Alias>alias</Alias></CreateAccessPointResult>`
	}
	return &http.Response{StatusCode: http.StatusOK, Status: http.StatusText(http.StatusOK), Header: http.Header{"Content-Type": []string{"application/xml"}}, Body: io.NopCloser(strings.NewReader(resp))}, nil
}

func (c *taggingHTTPClient) tagRequests() int {
	n := 0
	for _, r := range c.requests {
		if strings.Contains(r.path, "/tags/") {
			n++
		}
	}
	return n
}

func taggingManager(c *taggingHTTPClient) *resourceManager {
	return &resourceManager{
		sdkapi:  svcsdk.New(svcsdk.Options{HTTPClient: c, Region: "us-west-2"}),
		metrics: ackmetrics.NewMetrics("s3control"),
	}
}

func accessPointWithARN(arn string) *resource {
	r := &resource{ko: &svcapitypes.AccessPoint{}}
	r.ko.Spec.AccountID = aws.String(testAccountID)
	r.ko.Spec.Name = aws.String(testAPName)
	r.ko.Spec.Tags = []*svcapitypes.Tag{{Key: aws.String("declared"), Value: aws.String("yes")}}
	if arn != "" {
		a := ackv1alpha1.AWSResourceName(arn)
		r.ko.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{ARN: &a}
	}
	return r
}

func TestSdkFind_ReadsTagsForAGeneralPurposeAccessPoint(t *testing.T) {
	httpc := &taggingHTTPClient{accessPointBody: `<?xml version="1.0" encoding="UTF-8"?><GetAccessPointResult><Name>` + testAPName + `</Name><Bucket>test-bucket</Bucket><NetworkOrigin>Internet</NetworkOrigin><AccessPointArn>` + generalPurposeARN + `</AccessPointArn></GetAccessPointResult>`}
	latest, err := taggingManager(httpc).sdkFind(context.Background(), accessPointWithARN(""))
	require.NoError(t, err)
	assert.Equal(t, 1, httpc.tagRequests(), "ListTagsForResource must run for a general purpose access point")
	require.Len(t, latest.ko.Spec.Tags, 1)
	assert.Equal(t, "team", aws.ToString(latest.ko.Spec.Tags[0].Key))
}

// The tagging APIs are not documented for S3 on Outposts, so an Outposts ARN
// gets no tag read and the declared tags stay as they are.
func TestSdkFind_SkipsTagsForAnOutpostsAccessPoint(t *testing.T) {
	httpc := &taggingHTTPClient{accessPointBody: `<?xml version="1.0" encoding="UTF-8"?><GetAccessPointResult><Name>` + testAPName + `</Name><Bucket>` + outpostsBucketARN + `</Bucket><NetworkOrigin>VPC</NetworkOrigin></GetAccessPointResult>`}
	latest, err := taggingManager(httpc).sdkFind(context.Background(), accessPointWithARN(outpostsARN))
	require.NoError(t, err)
	assert.Equal(t, 0, httpc.tagRequests(), "no ListTagsForResource for an s3-outposts ARN")
	require.Len(t, latest.ko.Spec.Tags, 1)
	assert.Equal(t, "declared", aws.ToString(latest.ko.Spec.Tags[0].Key))
}

func TestSdkCreate_SendsTagsExceptForOutpostsBuckets(t *testing.T) {
	for _, tc := range []struct {
		name     string
		bucket   string
		wantTags bool
	}{
		{"general purpose bucket", "test-bucket", true},
		{"outposts bucket", outpostsBucketARN, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			httpc := &taggingHTTPClient{}
			desired := accessPointWithARN("")
			desired.ko.Spec.Bucket = aws.String(tc.bucket)
			_, err := taggingManager(httpc).sdkCreate(context.Background(), desired)
			require.NoError(t, err)
			require.NotEmpty(t, httpc.bodies)
			assert.Equal(t, tc.wantTags, strings.Contains(httpc.bodies[0], "<Tags>"), "create body: %s", httpc.bodies[0])
		})
	}
}

func TestTagsSupported(t *testing.T) {
	assert.True(t, tagsSupported(generalPurposeARN))
	assert.False(t, tagsSupported(outpostsARN))
	assert.False(t, tagsSupported(outpostsBucketARN))
	assert.True(t, tagsSupported("test-bucket"), "a bucket name is not an ARN and is never an Outposts bucket")
}
