	// The S3 Control tagging APIs are not documented for S3 on Outposts
	// access points; leave the tags out of the create for an Outposts bucket.
	if !tagsSupported(aws.ToString(desired.ko.Spec.Bucket)) {
		input.Tags = nil
	}
