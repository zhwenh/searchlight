package check_component_status

import (
	"encoding/json"
	"fmt"

	"github.com/appscode/searchlight/pkg/icinga"
	"github.com/spf13/cobra"
	core "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

type Request struct {
	masterURL      string
	kubeconfigPath string

	Selector      string
	ComponentName string
}

type objectInfo struct {
	Name   string `json:"name,omitempty"`
	Status string `json:"status,omitempty"`
}

type serviceOutput struct {
	Objects []*objectInfo `json:"objects,omitempty"`
	Message string        `json:"message,omitempty"`
}

func CheckComponentStatus(req *Request) (icinga.State, interface{}) {
	config, err := clientcmd.BuildConfigFromFlags(req.masterURL, req.kubeconfigPath)
	if err != nil {
		return icinga.Unknown, err
	}
	kubeClient := kubernetes.NewForConfigOrDie(config)

	var components []core.ComponentStatus
	if req.ComponentName != "" {
		comp, err := kubeClient.CoreV1().ComponentStatuses().Get(req.ComponentName, metav1.GetOptions{})
		if err != nil {
			return icinga.Unknown, err
		}
		components = []core.ComponentStatus{*comp}
	} else {
		comps, err := kubeClient.CoreV1().ComponentStatuses().List(metav1.ListOptions{
			LabelSelector: req.Selector,
		})
		if err != nil {
			return icinga.Unknown, err
		}
		components = comps.Items
	}

	objectInfoList := make([]*objectInfo, 0)
	for _, component := range components {
		for _, condition := range component.Conditions {
			if condition.Type == core.ComponentHealthy && condition.Status == core.ConditionFalse {
				objectInfoList = append(objectInfoList,
					&objectInfo{
						Name:   component.Name,
						Status: "Unhealthy",
					},
				)
			}
		}
	}

	if len(objectInfoList) == 0 {
		return icinga.OK, "All components are healthy"
	} else {
		output := &serviceOutput{
			Objects: objectInfoList,
			Message: fmt.Sprintf("%d unhealthy component(s)", len(objectInfoList)),
		}
		outputByte, err := json.MarshalIndent(output, "", "  ")
		if err != nil {
			return icinga.Unknown, err
		}
		return icinga.Critical, outputByte
	}
}

func NewCmd() *cobra.Command {
	var req Request

	cmd := &cobra.Command{
		Use:     "check_component_status",
		Short:   "Check Kubernetes Component Status",
		Example: "",

		Run: func(c *cobra.Command, args []string) {
			icinga.Output(CheckComponentStatus(&req))
		},
	}

	cmd.Flags().StringVar(&req.masterURL, "master", req.masterURL, "The address of the Kubernetes API server (overrides any value in kubeconfig)")
	cmd.Flags().StringVar(&req.kubeconfigPath, "kubeconfig", req.kubeconfigPath, "Path to kubeconfig file with authorization information (the master location is set by the master flag).")

	cmd.Flags().StringVarP(&req.Selector, "selector", "l", "", "Selector (label query) to filter on, supports '=', '==', and '!='.")
	cmd.Flags().StringVarP(&req.ComponentName, "componentName", "n", "", "Name of component which should be ready")
	return cmd
}
