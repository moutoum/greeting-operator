package main

import (
	"context"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	cli "github.com/urfave/cli/v2"
	apps "k8s.io/api/apps/v1"
	api "k8s.io/api/core/v1"
	kerror "k8s.io/apimachinery/pkg/api/errors"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

func main() {
	app := cli.NewApp()
	app.Name = "greeting-operator"
	app.Usage = "Automatically expose a greeting server"
	app.Flags = []cli.Flag{
		&cli.StringFlag{
			Name:    "image",
			Usage:   "Greeting server image",
			Value:   "greeting:latest",
			Aliases: []string{"i"},
			EnvVars: []string{"IMAGE"},
		},
		&cli.IntFlag{
			Name:    "port",
			Usage:   "Port used by the service",
			Value:   80,
			Aliases: []string{"p"},
			EnvVars: []string{"PORT"},
		},
		&cli.StringFlag{
			Name:    "namespace",
			Usage:   "Kubernetes namespace used to create resources",
			Value:   api.NamespaceDefault,
			Aliases: []string{"n"},
			EnvVars: []string{"NAMESPACE"},
		},
		&cli.UintFlag{
			Name:    "replicas",
			Usage:   "Number of greeting server replicas",
			Value:   1,
			Aliases: []string{"r"},
			EnvVars: []string{"REPLICAS"},
		},
		&cli.StringFlag{
			Name:    "name",
			Usage:   "Greeting name",
			Value:   "anonymous",
			EnvVars: []string{"NAME"},
		},
	}
	app.Action = run

	if err := app.Run(os.Args); err != nil {
		log.WithError(err).Fatal("Unable to start greeting operator")
	}
}

func run(cliCtx *cli.Context) error {
	config := &GreetingOperatorConfig{
		Image:     cliCtx.String("image"),
		Namespace: cliCtx.String("namespace"),
		Replicas:  cliCtx.Uint("replicas"),
		Name:      cliCtx.String("name"),
	}

	operator, err := NewGreetingOperator(config)
	if err != nil {
		return fmt.Errorf("creating operator: %w", err)
	}

	if err = operator.Start(cliCtx.Context); err != nil {
		return fmt.Errorf("start operator: %w", err)
	}

	return nil
}

// GreetingOperatorConfig is the configration required to create the GreetingOperator.
type GreetingOperatorConfig struct {
	// Image to use to create the greeting server.
	Image string
	// Port on which the greeting server is reachable.
	Port int
	// Namespace is which the resources are created.
	Namespace string
	// Number of greeting server replicas.
	Replicas uint
	// Name of the greeting server.
	Name string
}

// GreetingOperator exposes a greeting server on kubernetes.
type GreetingOperator struct {
	image     string
	port      int
	namespace string
	replicas  uint
	name      string
	client    *kubernetes.Clientset
}

// NewGreetingOperator creates a GreetingOperator linked to the current cluster.
func NewGreetingOperator(config *GreetingOperatorConfig) (*GreetingOperator, error) {
	cfg, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("in cluster config: %w", err)
	}

	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("new k8s client: %w", err)
	}

	op := GreetingOperator{
		image:     config.Image,
		port:      config.Port,
		namespace: config.Namespace,
		replicas:  config.Replicas,
		name:      config.Name,
		client:    client,
	}

	return &op, nil
}

// Start creates the k8s resources exposing a greeting server.
func (o *GreetingOperator) Start(ctx context.Context) error {
	if err := o.createNamespace(ctx); err != nil {
		return err
	}

	if err := o.createDeployment(ctx); err != nil {
		return err
	}

	if err := o.createService(ctx); err != nil {
		return err
	}

	return nil
}

func (o *GreetingOperator) createNamespace(ctx context.Context) error {
	log.WithField("namespace", o.namespace).Info("Creating namespace")

	namespace := &api.Namespace{
		ObjectMeta: meta.ObjectMeta{
			Name: o.namespace,
		},
	}

	if _, err := o.client.CoreV1().Namespaces().Create(ctx, namespace, meta.CreateOptions{}); err != nil {
		if !kerror.IsAlreadyExists(err) {
			return fmt.Errorf("create namespace: %w", err)
		}
	}

	log.WithField("namespace", o.namespace).Info("Namespace created")

	return nil
}

func (o *GreetingOperator) createDeployment(ctx context.Context) error {
	deploymentClient := o.client.AppsV1().Deployments(o.namespace)

	objMeta := meta.ObjectMeta{
		Name:   "greeting",
		Labels: map[string]string{"app": "greeting"},
	}

	podTpl := api.PodTemplateSpec{
		ObjectMeta: objMeta,
		Spec: api.PodSpec{
			Containers: []api.Container{{
				Name:  "greeting",
				Image: o.image,
				Ports: []api.ContainerPort{{
					Name:          "http",
					Protocol:      api.ProtocolTCP,
					ContainerPort: 80,
				}},
				Env: []api.EnvVar{{
					Name:  "NAME",
					Value: o.name,
				}},
				LivenessProbe: &api.Probe{
					ProbeHandler: api.ProbeHandler{
						HTTPGet: &api.HTTPGetAction{
							Path: "/health",
							Port: intstr.FromInt(80),
						},
					},
					TimeoutSeconds: 3,
				},
				ImagePullPolicy: api.PullNever,
			}},
			RestartPolicy: api.RestartPolicyAlways,
		},
	}

	var replicas int32 = 1
	greetingDeployment := &apps.Deployment{
		ObjectMeta: objMeta,
		Spec: apps.DeploymentSpec{
			Replicas: &replicas,
			Selector: &meta.LabelSelector{MatchLabels: map[string]string{"app": "greeting"}},
			Template: podTpl,
		},
	}

	log.Info("Creating deployment")

	var alreadyExists bool
	_, err := deploymentClient.Create(ctx, greetingDeployment, meta.CreateOptions{})
	if err != nil {
		if !kerror.IsAlreadyExists(err) {
			return fmt.Errorf("create deployment: %w", err)
		} else {
			alreadyExists = true
		}
	}

	if alreadyExists {
		log.Info("Deployment already exists, updating current")
		_, err = deploymentClient.Update(ctx, greetingDeployment, meta.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("update deployment: %w", err)
		}
	}

	log.Info("Deployment created")
	return nil
}

func (o *GreetingOperator) createService(ctx context.Context) error {
	serviceClient := o.client.CoreV1().Services(o.namespace)

	service := &api.Service{
		ObjectMeta: meta.ObjectMeta{Name: "greeting"},
		Spec: api.ServiceSpec{
			Selector: map[string]string{"app": "greeting"},
			Type:     api.ServiceTypeLoadBalancer,
			Ports: []api.ServicePort{{
				Name:       "http",
				Protocol:   api.ProtocolTCP,
				Port:       80,
				TargetPort: intstr.FromInt(o.port),
			}},
		},
	}

	var alreadyExists bool
	_, err := serviceClient.Create(ctx, service, meta.CreateOptions{})
	if err != nil {
		if !kerror.IsAlreadyExists(err) {
			return fmt.Errorf("create service: %w", err)
		} else {
			alreadyExists = true
		}
	}

	if alreadyExists {
		log.Info("Service already exists, updating current")
		_, err = serviceClient.Update(ctx, service, meta.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("update service: %w", err)
		}
	}

	log.Info("Service created")
	return nil
}
