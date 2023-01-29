package main

import (
	"context"
	"fmt"
	v12 "github.com/2456868764/operator/crd/pkg/apis/crd.jun.com/v1"
	clientset "github.com/2456868764/operator/crd/pkg/generated/clientset/versioned"
	"github.com/2456868764/operator/crd/pkg/generated/informers/externalversions"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"log"
)

func main() {
	//clientSet()
	restClient()

}

func restClient() {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		log.Fatalln(err)
	}

	config.APIPath = "apis"
	config.GroupVersion = &v12.GroupVersion
	config.NegotiatedSerializer = v12.Codec

	restClient, err := rest.RESTClientFor(config)
	if err != nil {
		log.Fatalln(err)
	}

	foo := v12.Foo{}
	restClient.Get().Namespace("default").Resource("foos").Name("example-jun-foo").Do(context.TODO()).Into(&foo)
	fmt.Printf("%+v", foo)
}

func clientSet() {
	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		log.Fatalln(err)
	}

	clientSet, err := clientset.NewForConfig(config)
	if err != nil {
		log.Fatalln(err)
	}

	list, err := clientSet.CrdV1().Foos("default").List(context.TODO(), v1.ListOptions{})
	if err != nil {
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
	factory.WaitForCacheSync(stopCh)

	<-stopCh

}
