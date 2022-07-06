package controllers

import (
	"context"
	"fmt"

	v1 "github.com/767829413/kubebuilder-demo/api/v1"

	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Finalizers去重
func GetUniqueFinalizersMap(finalizers []string) map[string]int {
	m := make(map[string]int, len(finalizers))
	for _, v := range finalizers {
		m[v] = 1
	}
	return m
}

// 通过实际请求状态来判断Pod是否存在
func IsRedisPodExist(podName string, redis *v1.Redis, client client.Client) bool {
	err := client.Get(context.Background(), types.NamespacedName{
		Name:      podName,
		Namespace: redis.Namespace,
	}, &coreV1.Pod{})
	return err == nil
}

// 创建redis Pod
func CreateRedisPod(podName string, client client.Client, redisConfig *v1.Redis, scheme *runtime.Scheme) error {
	newPod := &coreV1.Pod{}
	newPod.Name = podName
	newPod.Namespace = redisConfig.Namespace
	newPod.Spec.Containers = []coreV1.Container{
		{
			Name:            redisConfig.Name,
			Image:           "redis:5-alpine",
			ImagePullPolicy: coreV1.PullIfNotPresent,
			Ports: []coreV1.ContainerPort{
				{
					ContainerPort: int32(redisConfig.Spec.Port),
				},
			},
		},
	}
	err := controllerutil.SetControllerReference(redisConfig, newPod, scheme)
	if err != nil {
		return err
	}
	return client.Create(context.Background(), newPod)
}

// 获取自定义多副本的redis pod 名称
func GetRedisPodNames(redis *v1.Redis) []string {
	podNames := make([]string, redis.Spec.Num)
	for i := 0; i < redis.Spec.Num; i++ {
		podNames[i] = fmt.Sprintf("%s-%d", redis.Name, i)
	}
	return podNames
}
