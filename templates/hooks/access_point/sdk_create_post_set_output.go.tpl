// The policy and the tags are written by the update path, so a spec that
// carries either needs another reconcile right after the create.
if ko.Spec.Policy != nil || len(ko.Spec.Tags) > 0 {
	ackcondition.SetSynced(&resource{ko}, corev1.ConditionFalse, nil, nil)
}
