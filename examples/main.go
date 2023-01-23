package main

import (
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/workqueue"
)

func main() {
	//config
	//config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	//config.GroupVersion = &v1.SchemeGroupVersion
	//config.NegotiatedSerializer = scheme.Codecs
	//config.APIPath = "/api"
	//
	//if err != nil {
	//	panic(err.(any))
	//}
	////client
	//restClient, err := rest.RESTClientFor(config)
	//if err != nil {
	//	panic(err.(any))
	//}
	//
	////get data
	//pod := v1.Pod{}
	//err = restClient.Get().Namespace("default").Resource("pods").Name("reviews-v2-8454bb78d8-fddj6").Do(context.TODO()).Into(&pod)
	//if err!= nil {
	//	println(err)
	//} else {
	//	println(pod.Name)
	//}

	//config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	//if err != nil {
	//	println(err)
	//
	//}
	//clientSet, err := kubernetes.NewForConfig(config)
	//if err!= nil {
	//	println(err)
	//
	//}
	//coreV1 := clientSet.CoreV1()
	//pod, err := coreV1.Pods("default").Get(context.TODO(), "reviews-v2-8454bb78d8-fddj6", v1.GetOptions{})
	//if err != nil {
	//	println(err)
	//}
	//println(pod.Name)

	//config

	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		panic(err.(any))
	}

	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.(any))
	}

	queue := workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "controller")

	factory := informers.NewSharedInformerFactory(clientSet, 0)
	informer := factory.Core().V1().Pods().Informer()
	informer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			println("add function")
			key, _ := cache.MetaNamespaceKeyFunc(obj)
			queue.Add(key)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			println("update function")
			key, _ := cache.MetaNamespaceKeyFunc(newObj)
			queue.Add(key)
		},
		DeleteFunc: func(obj interface{}) {
			println("delete function")
			key, _ := cache.MetaNamespaceKeyFunc(obj)
			queue.Add(key)
		},
	})
	stopCh := make(chan struct{})
	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)
	<-stopCh

}
