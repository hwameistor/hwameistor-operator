package installhwamei

import (
	corev1 "k8s.io/api/core/v1"
)

var HostPathFileOrCreate = corev1.HostPathFileOrCreate
var HostPathDirectory = corev1.HostPathDirectory
var HostPathDirectoryOrCreate = corev1.HostPathDirectoryOrCreate
var HostPathTypeUnset = corev1.HostPathUnset
var SecurityContextPrivilegedTrue = true
var MountPropagationBidirectional = corev1.MountPropagationBidirectional
var TerminationGracePeriodSeconds30s = int64(30)