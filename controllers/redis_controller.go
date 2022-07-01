/*
Copyright 2022.

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

package controllers

import (
	"context"
	"fmt"

	myappv1 "github.com/767829413/kubebuilder-demo/api/v1"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// RedisReconciler reconciles a Redis object
type RedisReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=myapp.demo.kubebuilder.io,resources=redis,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=myapp.demo.kubebuilder.io,resources=redis/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=myapp.demo.kubebuilder.io,resources=redis/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Redis object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.1/pkg/reconcile
func (r *RedisReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)

	// TODO(user): your logic here
	redis := &myappv1.Redis{}
	// 获取自定义的Redis资源
	err := r.Get(ctx, req.NamespacedName, redis)
	if err != nil {
		return ctrl.Result{}, err
	}
	fmt.Println("redis obj:", redis)
	// 获取自定义redis资源需要创建的pod名称
	podNames := GetRedisPodNames(redis)
	fmt.Println("Pod names:", podNames)
	// 设置一个是否更新的判断
	upFlag := false

	// 删除自定义Redis资源的过程中,需要判断DeletionTimestamp(自动添加)
	// 如果有值表示删除,无值表示未删除
	if !redis.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, r.clearRedis(ctx, redis)
	}

	// 创建自定义资源对应的pod
	for _, podName := range podNames {
		if IsExist(podName, redis) {
			continue
		}
		err := CreateRedis(podName, r.Client, redis)
		if err != nil {
			fmt.Println("create pod failue,", err)
			return ctrl.Result{}, err
		}
		// 如果创建的pod不在finalizers中，则添加
		redis.Finalizers = append(redis.Finalizers, podName)
		upFlag = true
	}

	// 更细自定义资源 Redis 状态
	if upFlag {
		r.Client.Update(ctx, redis)
	}

	return ctrl.Result{}, nil
}

// 删除自定义Redis资源和其创建的Pod逻辑
func (r *RedisReconciler) clearRedis(ctx context.Context, redis *myappv1.Redis) error {
	for _, finalizer := range redis.Finalizers {
		// 通过finalizer来批量删除自定义Redis资源创建的Pod
		err := r.Client.Delete(ctx, &coreV1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      finalizer,
				Namespace: redis.Namespace,
			},
		})
		if err != nil {
			return err
		}
	}
	// 只有设置 Finalizers 为空才能真正删除自定义资源Redis
	redis.Finalizers = []string{}
	return r.Client.Update(ctx, redis)
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&myappv1.Redis{}).
		Complete(r)
}
