package service

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/google/wire"
	"github.com/jmoiron/sqlx"
	"github.com/x-thooh/delay/internal/service/delay"
	"github.com/x-thooh/delay/internal/service/example"
	"github.com/x-thooh/delay/internal/service/storage"
	"github.com/x-thooh/delay/pkg/log"
	"github.com/x-thooh/delay/pkg/util"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

var ProviderSetService = wire.NewSet(
	delay.New,
	example.New,
	RegisterStorage,
)

func RegisterStorage(
	cfg *storage.Config,
	lg log.Logger,
	db *sqlx.DB,
) (*storage.Storage, error) {
	ordinal, err := GetCurrentPodOrdinal()
	if err != nil {
		return nil, err
	}
	cfg.Node = ordinal
	s, err := storage.New(cfg, lg, db)
	if err != nil {
		return nil, err
	}

	if IsInPod() {
		var (
			client *kubernetes.Clientset
		)
		// 客户端
		client, err = getClientSet()
		if err != nil {
			return nil, err
		}

		// SS容器
		ssp := NewStatefulSetPod()
		ctx := context.Background()
		go func() {
			if rev := recover(); rev != nil {
				lg.Error(ctx, "WatchOrdinal error", "rec", rev)
			}
			// 被动监听
			if err = ssp.WatchOrdinal(ctx, client, cfg.NameSpace, cfg.StsName, func(ordinals *util.SafeMap[int, struct{}]) {
				s.SetNodes(GetSortOrdinal(ordinals))
			}); err != nil {
				lg.Error(ctx, "WatchOrdinal error", "err", err)
			}
		}()

		//  主动拉取
		if err = s.ScheduleFunc(cfg.NodeInterval, func(ctx context.Context) {
			m, err := ssp.GetOrdinal(ctx, client, cfg.NameSpace, cfg.StsName)
			if err != nil {
				lg.Error(ctx, "GetOrdinal error", err)
			}
			s.SetNodes(GetSortOrdinal(m))
		}); err != nil {
			return nil, err
		}
	}

	return s, nil
}

type StatefulSetPod struct {
	ordinals *util.SafeMap[int, struct{}]
}

func NewStatefulSetPod() *StatefulSetPod {
	return &StatefulSetPod{ordinals: util.NewSafeMap[int, struct{}]()}
}

// IsInPod 返回 true 表示程序运行在 Kubernetes Pod 内
func IsInPod() bool {
	host := os.Getenv("KUBERNETES_SERVICE_HOST")
	port := os.Getenv("KUBERNETES_SERVICE_PORT")
	return host != "" && port != ""
}

func getClientSet() (*kubernetes.Clientset, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return clientSet, nil
}

func (ss *StatefulSetPod) GetOrdinal(ctx context.Context, cs *kubernetes.Clientset, namespace, stsName string) (*util.SafeMap[int, struct{}], error) {
	// 获取 StatefulSet 的 label selector
	sts, err := cs.AppsV1().StatefulSets(namespace).Get(ctx, stsName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("get statefulset %s: %w", stsName, err)
	}

	// 获取 Pod 列表
	pods, err := cs.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{
		// 标签筛选
		LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{MatchLabels: sts.Spec.Selector.MatchLabels}),
	})
	if err != nil {
		return nil, fmt.Errorf("list pods: %w", err)
	}

	for _, p := range pods.Items {
		parts := strings.Split(p.Name, "-")
		nStr := parts[len(parts)-1]
		n, err := strconv.Atoi(nStr)
		if err != nil {
			continue
		}
		ss.ordinals.Set(n, struct{}{})
	}

	return ss.ordinals, nil
}

func (ss *StatefulSetPod) WatchOrdinal(ctx context.Context, cs *kubernetes.Clientset, namespace, stsName string, fn func(ordinals *util.SafeMap[int, struct{}])) error {
	// 获取 StatefulSet 的 label selector
	sts, err := cs.AppsV1().StatefulSets(namespace).Get(ctx, stsName, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("get statefulset %s: %w", stsName, err)
	}

	// Watch Pod 列表
	watchInterface, err := cs.CoreV1().Pods(namespace).Watch(ctx, metav1.ListOptions{
		LabelSelector: metav1.FormatLabelSelector(&metav1.LabelSelector{MatchLabels: sts.Spec.Selector.MatchLabels}),
	})
	if err != nil {
		return fmt.Errorf("watch pods: %w", err)
	}

	ch := watchInterface.ResultChan()
	for event := range ch {
		pod, ok := event.Object.(*v1.Pod)
		if !ok {
			fmt.Println("unexpected type")
			continue
		}

		ordinal, err := GetOrdinal(pod)
		if err != nil {
			return err
		}

		switch event.Type {
		case watch.Added:
			ss.ordinals.Set(ordinal, struct{}{})
			fn(ss.ordinals)
		case watch.Deleted, watch.Error:
			ss.ordinals.Delete(ordinal)
			fn(ss.ordinals)
		case watch.Modified:
		}
	}

	return nil
}

func GetOrdinal(p *v1.Pod) (int, error) {
	parts := strings.Split(p.Name, "-")
	nStr := parts[len(parts)-1]
	n, err := strconv.Atoi(nStr)
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func GetSortOrdinal(m *util.SafeMap[int, struct{}]) (ret []int) {
	ret = make([]int, 0, len(m.Keys()))
	for _, item := range m.Keys() {
		ret = append(ret, item)
	}
	sort.Ints(ret)
	return
}

// GetCurrentPodOrdinal 解析当前 Pod 名称最后的数字
func GetCurrentPodOrdinal() (int, error) {
	if IsInPod() {
		// 从环境变量获取 Pod 名称（通常由 downward API 注入）
		podName := os.Getenv("POD_NAME")
		if podName == "" {
			// 如果没有注入环境变量，可以尝试 hostname
			podName, _ = os.Hostname()
		}

		parts := strings.Split(podName, "-")
		nStr := parts[len(parts)-1]

		return strconv.Atoi(nStr)
	}
	return 0, nil
}
