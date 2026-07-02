if ackcompare.IsNotNil(a.ko.Spec.Policy) && ackcompare.IsNotNil(b.ko.Spec.Policy) {
	equal, err := ackcompare.IAMPolicyDocumentEqual(*a.ko.Spec.Policy, *b.ko.Spec.Policy)
	if err != nil || !equal {
		delta.Add("Spec.Policy", a.ko.Spec.Policy, b.ko.Spec.Policy)
	}
} else if ackcompare.IsNotNil(a.ko.Spec.Policy) != ackcompare.IsNotNil(b.ko.Spec.Policy) {
	delta.Add("Spec.Policy", a.ko.Spec.Policy, b.ko.Spec.Policy)
}
