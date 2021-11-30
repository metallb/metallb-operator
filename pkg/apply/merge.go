package apply

import (
	"github.com/pkg/errors"
	uns "k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// MergeMetadataForUpdate merges the read-only fields of metadata.
// This is to be able to do a a meaningful comparison in apply,
// since objects created on runtime do not have these fields populated.
func mergeMetadataForUpdate(current, updated *uns.Unstructured) error {
	updated.SetCreationTimestamp(current.GetCreationTimestamp())
	updated.SetSelfLink(current.GetSelfLink())
	updated.SetGeneration(current.GetGeneration())
	updated.SetUID(current.GetUID())
	updated.SetResourceVersion(current.GetResourceVersion())
	updated.SetManagedFields(current.GetManagedFields())
	updated.SetFinalizers(current.GetFinalizers())

	mergeAnnotations(current, updated)
	mergeLabels(current, updated)

	return nil
}

const (
	MetalLBConfigMap = "config"
)

// MergeObjectForUpdate prepares a "desired" object to be updated.
// Some objects, such as Deployments and Services require
// some semantic-aware updates
func MergeObjectForUpdate(current, updated *uns.Unstructured) error {
	if err := mergeDeploymentForUpdate(current, updated); err != nil {
		return err
	}

	if err := mergeServiceForUpdate(current, updated); err != nil {
		return err
	}

	if err := mergeServiceAccountForUpdate(current, updated); err != nil {
		return err
	}

	// For all object types, merge metadata.
	// Run this last, in case any of the more specific merge logic has
	// changed "updated"
	_ = mergeMetadataForUpdate(current, updated)

	return nil
}

const (
	deploymentRevisionAnnotation = "deployment.kubernetes.io/revision"
)

// mergeDeploymentForUpdate updates Deployment objects.
// We merge annotations, keeping ours except the Deployment Revision annotation.
func mergeDeploymentForUpdate(current, updated *uns.Unstructured) error {
	gvk := updated.GroupVersionKind()
	if gvk.Group == "apps" && gvk.Kind == "Deployment" {

		// Copy over the revision annotation from current up to updated
		// otherwise, updated would win, and this annotation is "special" and
		// needs to be preserved
		curAnnotations := current.GetAnnotations()
		updatedAnnotations := updated.GetAnnotations()
		if updatedAnnotations == nil {
			updatedAnnotations = map[string]string{}
		}

		anno, ok := curAnnotations[deploymentRevisionAnnotation]
		if ok {
			updatedAnnotations[deploymentRevisionAnnotation] = anno
		}

		updated.SetAnnotations(updatedAnnotations)
	}

	return nil
}

// mergeServiceForUpdate ensures the ClusterIP/IPFamily is never modified
func mergeServiceForUpdate(current, updated *uns.Unstructured) error {
	gvk := updated.GroupVersionKind()
	if gvk.Group == "" && gvk.Kind == "Service" {
		clusterIP, found, err := uns.NestedString(current.Object, "spec", "clusterIP")
		if err != nil {
			return err
		}
		if found {
			err = uns.SetNestedField(updated.Object, clusterIP, "spec", "clusterIP")
			if err != nil {
				return err
			}
		}

		clusterIPs, found, err := uns.NestedStringSlice(current.Object, "spec", "clusterIPs")
		if err != nil {
			return err
		}
		if found {
			err = uns.SetNestedStringSlice(updated.Object, clusterIPs, "spec", "clusterIPs")
			if err != nil {
				return err
			}
		}

		ipFamilies, found, err := uns.NestedStringSlice(current.Object, "spec", "ipFamilies")
		if err != nil {
			return err
		}
		if found {
			err = uns.SetNestedStringSlice(updated.Object, ipFamilies, "spec", "ipFamilies")
			if err != nil {
				return err
			}
		}

		ipFamilyPolicy, foundOld, err := uns.NestedString(current.Object, "spec", "ipFamilyPolicy")
		if err != nil {
			return err
		}
		_, foundNew, err := uns.NestedString(updated.Object, "spec", "ipFamilyPolicy")
		if err != nil {
			return err
		}
		if foundOld && !foundNew {
			err = uns.SetNestedField(updated.Object, ipFamilyPolicy, "spec", "ipFamilyPolicy")
			if err != nil {
				return err
			}
		}

	}

	return nil
}

// mergeServiceAccountForUpdate copies secrets from current to updated.
// This is intended to preserve the auto-generated token.
// Right now, we just copy current to updated and don't support supplying
// any secrets ourselves.
func mergeServiceAccountForUpdate(current, updated *uns.Unstructured) error {
	gvk := updated.GroupVersionKind()
	if gvk.Group == "" && gvk.Kind == "ServiceAccount" {
		curSecrets, ok, err := uns.NestedSlice(current.Object, "secrets")
		if err != nil {
			return err
		}

		if ok {
			_ = uns.SetNestedField(updated.Object, curSecrets, "secrets")
		}

		curImagePullSecrets, ok, err := uns.NestedSlice(current.Object, "imagePullSecrets")
		if err != nil {
			return err
		}
		if ok {
			_ = uns.SetNestedField(updated.Object, curImagePullSecrets, "imagePullSecrets")
		}
	}
	return nil
}

// mergeAnnotations copies over any annotations from current to updated,
// with updated winning if there's a conflict
func mergeAnnotations(current, updated *uns.Unstructured) {
	updatedAnnotations := updated.GetAnnotations()
	curAnnotations := current.GetAnnotations()

	if curAnnotations == nil {
		curAnnotations = map[string]string{}
	}

	for k, v := range updatedAnnotations {
		curAnnotations[k] = v
	}

	if len(curAnnotations) != 0 {
		updated.SetAnnotations(curAnnotations)
	}
}

// mergeLabels copies over any labels from current to updated,
// with updated winning if there's a conflict
func mergeLabels(current, updated *uns.Unstructured) {
	updatedLabels := updated.GetLabels()
	curLabels := current.GetLabels()

	if curLabels == nil {
		curLabels = map[string]string{}
	}

	for k, v := range updatedLabels {
		curLabels[k] = v
	}

	if len(curLabels) != 0 {
		updated.SetLabels(curLabels)
	}
}

// IsObjectSupported rejects objects with configurations we don't support.
// This catches ServiceAccounts with secrets, which is valid but we don't
// support reconciling them.
func IsObjectSupported(obj *uns.Unstructured) error {
	gvk := obj.GroupVersionKind()

	// We cannot create ServiceAccounts with secrets because there's currently
	// no need and the merging logic is complex.
	// If you need this, please file an issue.
	if gvk.Group == "" && gvk.Kind == "ServiceAccount" {
		secrets, ok, err := uns.NestedSlice(obj.Object, "secrets")
		if err != nil {
			return err
		}

		if ok && len(secrets) > 0 {
			return errors.Errorf("cannot create ServiceAccount with secrets")
		}
	}

	return nil
}
