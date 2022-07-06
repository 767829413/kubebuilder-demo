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

	v1 "github.com/767829413/kubebuilder-demo/api/v1"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// RedisReconciler reconciles a Redis object
type RedisReconciler struct {
	client.Client
	Scheme        *runtime.Scheme
	EventRecorder record.EventRecorder // 增加事件记录
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
	redis := &v1.Redis{}
	// 获取自定义的Redis资源
	err := r.Get(ctx, req.NamespacedName, redis)
	if err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	// 获取自定义redis资源需要创建的pod名称
	podNames := GetRedisPodNames(redis)
	// 设置一个是否更新的判断
	upFlag := false

	// 删除自定义Redis资源的过程中,需要判断DeletionTimestamp(自动添加)
	// 如果有值表示删除,无值表示未删除
	// 如果缩容时候需要删除相应pod数目
	// len(redis.Finalizers) > redis.Spec.Num 考虑刚开始创建时候 len(redis.Finalizers)必定为0
	// 后续循环的时候,只要没有扩容或者缩容,必定 len(redis.Finalizers) == redis.Spec.Num
	if !redis.DeletionTimestamp.IsZero() || len(redis.Finalizers) > redis.Spec.Num {
		return ctrl.Result{}, r.clearRedis(ctx, redis)
	}
	// 防止重复pod进入Finalizers
	m := GetUniqueFinalizersMap(redis.Finalizers)
	// 创建自定义资源对应的pod
	for _, podName := range podNames {
		if IsRedisPodExist(podName, redis, r.Client) {
			continue
		}
		err := CreateRedisPod(podName, r.Client, redis, r.Scheme)
		if err != nil {
			fmt.Println("create pod failue,", err)
			return ctrl.Result{}, err
		}
		// 如果创建的pod不在finalizers中，则添加
		if _, ok := m[podName]; !ok {
			redis.Finalizers = append(redis.Finalizers, podName)
			upFlag = true
		}
	}

	// 更细自定义资源 Redis 状态
	if upFlag {
		r.EventRecorder.Event(redis, coreV1.EventTypeNormal, "Upgrade", "Capacity expansion and reduction")
		//更新status状态值
		redis.Status.Replicas = len(redis.Finalizers)
		err := r.Status().Update(ctx, redis)
		if err != nil {
			return ctrl.Result{}, err
		}
		err = r.Client.Update(ctx, redis)
		if err != nil {
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// 删除自定义Redis资源和其创建的Pod逻辑
func (r *RedisReconciler) clearRedis(ctx context.Context, redis *v1.Redis) error {
	// 副本数与当前Pod数量的差值就是需要删除的数量
	// 如果相等,删除全部
	// 1. len(redis.Finalizers) > redis.Spec.Num 删除差值数量Pod
	// 2. len(redis.Finalizers) == redis.Spec.Num 删除全部数量Pod
	// 3. len(redis.Finalizers) < redis.Spec.Num 错误的数量,不建议操作
	// 删除后项
	var deletedPodNames []string
	diffNum := len(redis.Finalizers) - redis.Spec.Num
	if diffNum == 0 {
		deletedPodNames = make([]string, len(redis.Finalizers))
		copy(deletedPodNames, redis.Finalizers)
		// 只有设置 Finalizers 为空才能真正删除自定义资源Redis
		redis.Finalizers = []string{}
	} else if diffNum > 0 {
		deletedPodNames = redis.Finalizers[redis.Spec.Num:]
		redis.Finalizers = redis.Finalizers[:redis.Spec.Num]
	}

	for _, finalizer := range deletedPodNames {
		// 通过finalizer来批量删除自定义Redis资源创建的Pod
		err := r.Client.Delete(ctx, &coreV1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      finalizer,
				Namespace: redis.Namespace,
			},
		})
		err = client.IgnoreNotFound(err)
		if err != nil {
			return err
		}
	}
	err := r.Client.Update(ctx, redis)
	if err != nil {
		return err
	}
	err = r.Status().Update(ctx, redis)
	return err
}

// Pod删除时的回调
func (r *RedisReconciler) podDelHandler(event event.DeleteEvent, limitingInterface workqueue.RateLimitingInterface) {
	fmt.Println("deleted pod name is :", event.Object.GetName())
	// 取得OwnerReference,如果自定义资源Kind和Name匹配,则触发reconcile.Request，并加入到等待队列
	for _, ownerReference := range event.Object.GetOwnerReferences() {
		if ownerReference.Kind == "Redis" && ownerReference.Name == "redis-sample" {
			limitingInterface.Add(reconcile.Request{
				NamespacedName: types.NamespacedName{
					Namespace: event.Object.GetNamespace(),
					Name:      ownerReference.Name,
				},
			})
		}
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *RedisReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1.Redis{}).
		Watches(&source.Kind{
			Type: &coreV1.Pod{},
		}, handler.Funcs{
			DeleteFunc: r.podDelHandler,
		}).
		Complete(r)
}
