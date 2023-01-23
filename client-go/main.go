package main

import (
	"github.com/2456868764/operator/client-go/pkg"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"log"
)

func main()  {
	//conifg
	//client
	//informer
	// add event handler
	//informer.start

	config, err := clientcmd.BuildConfigFromFlags("", clientcmd.RecommendedHomeFile)
	if err != nil {
		clusterConfig, err := rest.InClusterConfig()
		if err!= nil {
			log.Fatalln("can't get config")

		}
		config = clusterConfig
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err!= nil {
		log.Fatalln("can't create client")
	}

	factory := informers.NewSharedInformerFactory(clientset, 0)
	serviceInformer := factory.Core().V1().Services()
	ingressInformer := factory.Networking().V1().Ingresses()
	controller := pkg.NewController(clientset, serviceInformer, ingressInformer)
	stopCh := make(chan struct{})
	factory.Start(stopCh)
	factory.WaitForCacheSync(stopCh)
	controller.Run(stopCh)
}
