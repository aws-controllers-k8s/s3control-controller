if delta.DifferentAt("Spec.Policy") {
	if desired.ko.Spec.Policy != nil && *desired.ko.Spec.Policy != "" {
		policyInput := &svcsdk.PutAccessPointPolicyInput{
			AccountId: desired.ko.Spec.AccountID,
			Name:      desired.ko.Spec.Name,
			Policy:    desired.ko.Spec.Policy,
		}
		_, err = rm.sdkapi.PutAccessPointPolicy(ctx, policyInput)
		if err != nil {
			return nil, err
		}
	} else {
		deleteInput := &svcsdk.DeleteAccessPointPolicyInput{
			AccountId: desired.ko.Spec.AccountID,
			Name:      desired.ko.Spec.Name,
		}
		_, err = rm.sdkapi.DeleteAccessPointPolicy(ctx, deleteInput)
		if err != nil {
			var awsErr smithy.APIError
			if !errors.As(err, &awsErr) || awsErr.ErrorCode() != "NoSuchAccessPointPolicy" {
				return nil, err
			}
		}
	}
}
