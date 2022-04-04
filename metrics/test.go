package main

import (
	"fmt"
	"github.com/RHEcosystemAppEng/dbaas-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"os"

	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var kubeconfig string

func init() {
	fmt.Printf("init...")
	kubeconfig = os.Getenv("KUBECONFIG")
	//flag.StringVar(&kubeconfig, "kubeconfig", "", "path to Kubernetes config file")
	//flag.Parse()
}

func main() {
	fmt.Printf("Starting...")
	var config *rest.Config
	var err error

	if kubeconfig == "" {
		log.Printf("using in-cluster configuration")
		config, err = rest.InClusterConfig()
	} else {
		log.Printf("using configuration from '%s'", kubeconfig)
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}

	if err != nil {
		panic(err)
	}

	v1alpha1.AddToScheme(scheme.Scheme)

	clientSet, err := v1alpha1.NewForConfig(config)
	if err != nil {
		panic(err)
	}

	projects, err := clientSet.DbaaSPlatform("openshift-dbaas-operator").List(metav1.ListOptions{})
	if err != nil {
		panic(err)
	}

	fmt.Printf("projects found: %+v\n", projects)
}
