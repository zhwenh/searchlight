package util

import (
	"encoding/json"
	"fmt"

	"github.com/appscode/kutil"
	api "github.com/appscode/searchlight/apis/monitoring/v1alpha1"
	cs "github.com/appscode/searchlight/client/clientset/versioned/typed/monitoring/v1alpha1"
	"github.com/golang/glog"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/jsonmergepatch"
	"k8s.io/apimachinery/pkg/util/wait"
)

func CreateOrPatchSearchlightPlugin(c cs.MonitoringV1alpha1Interface, meta metav1.ObjectMeta, transform func(alert *api.SearchlightPlugin) *api.SearchlightPlugin) (*api.SearchlightPlugin, kutil.VerbType, error) {
	cur, err := c.SearchlightPlugins().Get(meta.Name, metav1.GetOptions{})
	if kerr.IsNotFound(err) {
		glog.V(3).Infof("Creating SearchlightPlugin %s/%s.", meta.Namespace, meta.Name)
		out, err := c.SearchlightPlugins().Create(transform(&api.SearchlightPlugin{
			TypeMeta: metav1.TypeMeta{
				Kind:       "SearchlightPlugin",
				APIVersion: api.SchemeGroupVersion.String(),
			},
			ObjectMeta: meta,
		}))
		return out, kutil.VerbCreated, err
	} else if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	return PatchSearchlightPlugin(c, cur, transform)
}

func PatchSearchlightPlugin(c cs.MonitoringV1alpha1Interface, cur *api.SearchlightPlugin, transform func(*api.SearchlightPlugin) *api.SearchlightPlugin) (*api.SearchlightPlugin, kutil.VerbType, error) {
	return PatchSearchlightPluginObject(c, cur, transform(cur.DeepCopy()))
}

func PatchSearchlightPluginObject(c cs.MonitoringV1alpha1Interface, cur, mod *api.SearchlightPlugin) (*api.SearchlightPlugin, kutil.VerbType, error) {
	curJson, err := json.Marshal(cur)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	modJson, err := json.Marshal(mod)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}

	patch, err := jsonmergepatch.CreateThreeWayJSONMergePatch(curJson, modJson, curJson)
	if err != nil {
		return nil, kutil.VerbUnchanged, err
	}
	if len(patch) == 0 || string(patch) == "{}" {
		return cur, kutil.VerbUnchanged, nil
	}
	glog.V(3).Infof("Patching SearchlightPlugin %s/%s with %s.", cur.Namespace, cur.Name, string(patch))
	out, err := c.SearchlightPlugins().Patch(cur.Name, types.MergePatchType, patch)
	return out, kutil.VerbPatched, err
}

func TryUpdateSearchlightPlugin(c cs.MonitoringV1alpha1Interface, meta metav1.ObjectMeta, transform func(*api.SearchlightPlugin) *api.SearchlightPlugin) (result *api.SearchlightPlugin, err error) {
	attempt := 0
	err = wait.PollImmediate(kutil.RetryInterval, kutil.RetryTimeout, func() (bool, error) {
		attempt++
		cur, e2 := c.SearchlightPlugins().Get(meta.Name, metav1.GetOptions{})
		if kerr.IsNotFound(e2) {
			return false, e2
		} else if e2 == nil {
			result, e2 = c.SearchlightPlugins().Update(transform(cur.DeepCopy()))
			return e2 == nil, nil
		}
		glog.Errorf("Attempt %d failed to update SearchlightPlugin %s/%s due to %v.", attempt, cur.Namespace, cur.Name, e2)
		return false, nil
	})

	if err != nil {
		err = fmt.Errorf("failed to update SearchlightPlugin %s/%s after %d attempts due to %v", meta.Namespace, meta.Name, attempt, err)
	}
	return
}
