package main

import (
	"context"
	clientset "github.com/2456868764/operator/crd/pkg/generated/clientset/versioned"
	"github.com/2456868764/operator/crd/pkg/generated/informers/externalversions"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"log"
)

func main() {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		log.Fatalln(err)
	}

	clientSet, err := clientset.NewForConfig(config)
	if err != nil {
		log.Fatalln(err)
	}

	list, err := clientSet.CrdV1().Foos("default").List(context.TODO(), v1.ListOptions{})
	if err !=nil {
		log.Fatalln(err)
	}

	for _, foo := range list.Items {
		println(foo.Name)
	}

	factory := externalversions.NewSharedInformerFactory(clientSet, 0)
	factory.Crd().V1().Foos().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			log.Printf("add object:%v", obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			log.Printf("update object:%v", newObj)
		},
		
		DeleteFunc: func(obj interface{}) {
			log.Printf("delete object:%v", obj)
		},
		
	})



	stopCh := make(chan struct{})

	factory.Start(stopCh)

	<- stopCh


}
