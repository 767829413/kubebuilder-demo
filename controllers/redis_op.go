package controllers

import (
	"context"
	"fmt"

	v1 "github.com/767829413/kubebuilder-demo/api/v1"

	coreV1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// 利用finalizer来进行资源关联的删除
func IsExist(podName string, redis *v1.Redis) bool {
	for _, finalizer := range redis.Finalizers {
		if podName == finalizer {
			return true
		}
	}
	return false
}

// 创建redis Pod
func CreateRedis(podName string, client client.Client, redisConfig *v1.Redis) error {
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
