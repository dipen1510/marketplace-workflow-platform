/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controller

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	argowfv1alpha1 "github.com/argoproj/argo-workflows/v4/pkg/apis/workflow/v1alpha1"
	workflowv1alpha1 "github.com/dipen1510/marketplace-workflow-platform/controller/api/v1alpha1"

	corev1 "k8s.io/api/core/v1"
)

var _ = Describe("MarketplaceWorkflow Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			resourceName      = "test-resource"
			resourceNamespace = "default"
		)

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: resourceNamespace,
		}
		marketplaceworkflow := &workflowv1alpha1.MarketplaceWorkflow{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind MarketplaceWorkflow")
			err := k8sClient.Get(ctx, typeNamespacedName, marketplaceworkflow)
			if err != nil && errors.IsNotFound(err) {
				resource := &workflowv1alpha1.MarketplaceWorkflow{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},

					Spec: workflowv1alpha1.MarketplaceWorkflowSpec{

						Schedules: []string{
							"*/5 * * * *",
						},

						Timezone: "UTC",

						Suspend: false,

						ConcurrencyPolicy: workflowv1alpha1.ConcurrencyForbid,

						WorkflowTemplateRef: workflowv1alpha1.WorkflowTemplateReference{
							Name: "test-workflow-template",
						},

						Parameters: []workflowv1alpha1.WorkflowParameter{
							{
								Name:  "failureMode",
								Value: "success",
							},
						},
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &workflowv1alpha1.MarketplaceWorkflow{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance MarketplaceWorkflow")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &MarketplaceWorkflowReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})
})

func createWorkflowTemplate(
	ctx context.Context,
	name string,
	namespace string,
) {

	template := &argowfv1alpha1.WorkflowTemplate{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},

		Spec: argowfv1alpha1.WorkflowSpec{
			Entrypoint: "test",

			Templates: []argowfv1alpha1.Template{
				{
					Name: "test",

					Container: &corev1.Container{
						Image: "alpine:3.22",

						Command: []string{
							"echo",
						},

						Args: []string{
							"hello",
						},
					},
				},
			},
		},
	}

	Expect(
		k8sClient.Create(
			ctx,
			template,
		),
	).To(Succeed())
}
