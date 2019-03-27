package v1

import (
	"time"
	"fmt"
	"k8s-crd-operater/kevin/v1"
	"kube-deploy/web/reqBody"
	"kube-deploy/web/service"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/kubernetes"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s-crd-website/config"
)
var webSites   = map[string]string{}//应该使用etcd或其它数据库存储
func CheckWebSite()  {
	fmt.Println("start  loop")
	for true {
		// 获取所有的website
		var list = v1.List{Base:getWebSiteConfig()}
		result := list.Operate().Items
		tmpWebSites := map[string]string{}

		for _, item := range result {
			webSites[item.GetName()] = "1"
			tmpWebSites[item.GetName()] = "1"
			var itemName = item.GetName()
			var itemVersion = item.GetResourceVersion()
			//查看该website是否存在通过label关联的deploy或service
			deploys := v1.List{Base:getDeployConfig(),Labels:"app="+itemName}.Operate()
			//如果不存在，则新建，如果存在，则过
			if len(deploys.Items)==0{
				fmt.Println("不存在，创建服务")
				spec := item.Object["spec"].(map[string]interface{})

				create(itemName,spec["image"].(string),itemVersion)

			}else{
				//比较版本
				fmt.Println("已存在，查看版本")
				//deploys = v1.List{Base:getDeployConfig(),Labels:"appVersion="+itemName+"_"+itemVersion}.Operate()//不可采用该方式
				for index := range deploys.Items {
					annos := deploys.Items[index].GetAnnotations();
					fmt.Println(itemVersion)
					fmt.Println(deploys.Items[index].GetName(),annos)
					if itemVersion!=annos["itemVersion"]{
						fmt.Println("版本不一致")
						spec := item.Object["spec"].(map[string]interface{})

						update(itemName,spec["image"].(string),itemVersion)
					}
					break;
				}
			}

		}

		if len(webSites)> len(tmpWebSites){
			for site := range webSites {
				if tmpWebSites[site] == "" && webSites[site] != ""{
					//删除该deploy & service
					fmt.Println("删除website")
					delete(site)
					webSites[site] = ""
				}
			}
		}
		time.Sleep(time.Second*10)
	}

	fmt.Println("end  loop")

}



func getWebSiteConfig() v1.Base{
	return v1.Base{
		//Config:"/Users/liukai/go/src/k8s-crd-website/resources/read-test-kubeconfig",
		Group:"kevincrd.k8s.io",
		Version:"v1",
		Resource:"websites",
		Namespaces:"default",
		MasterUrl:config.MasterUrl,
	}
}


func getDeployConfig() v1.Base{
	return v1.Base{
		//Config:"/Users/liukai/go/src/k8s-crd-website/resources/read-test-kubeconfig",
		Group:"extensions",
		Version:"v1beta1",
		Resource:"deployments",
		Namespaces:"default",
		MasterUrl:config.MasterUrl,
	}
}


// 获取client
func getClient()*kubernetes.Clientset{
	config, _ :=clientcmd.BuildConfigFromFlags("http://localhost:8088/", "")
	clientset, _ := kubernetes.NewForConfig(config)
	return clientset

}

// create deploy & service
func create(itemName string,image string,itemVersion string){
	//不存在，创建deploy及service
	annos := map[string]string{
		"itemVersion":itemVersion,
	}
	request := reqBody.ServiceRequest{ServiceName:itemName,Image:image,Anno:annos,Namespace:"default",InstanceNum:1}

	deploymentsClient := getClient().AppsV1beta1().Deployments(request.Namespace)

	_, err := deploymentsClient.Create(service.GetDeployment(request))
	if err!=nil{
		panic(err)
	}


	ports := []apiv1.ServicePort{};
	port := apiv1.ServicePort{Port:8080,Name:"tcp",TargetPort:intstr.FromInt(8080)}
	ports = append(ports, port)
	getClient().CoreV1().Services(request.Namespace).Create(&apiv1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: request.ServiceName,
		},
		Spec: apiv1.ServiceSpec{
			Type:     apiv1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app": request.ServiceName,
			},
			Ports: ports,
		},
	})
}

func delete(itemName string){
	deploymentsClient := getClient().AppsV1beta1().Deployments("default")

	deletePolicy := metav1.DeletePropagationForeground//如果不添加该policy，则删除时，只删除deployment,不会删除对应的rs
	err := deploymentsClient.Delete(itemName,&metav1.DeleteOptions{PropagationPolicy:&deletePolicy})
	//err := deploymentsClient.Delete(itemName,&metav1.DeleteOptions{})
	if err!=nil{
		panic(err)
	}

	serviceClient := getClient().CoreV1().Services("default")
	err = serviceClient.Delete(itemName,&metav1.DeleteOptions{})
	if err!=nil{
		panic(err)
	}

}


func update(itemName string,image string,itemVersion string){
	annos := map[string]string{
		"itemVersion":itemVersion,
	}

	request := reqBody.ServiceRequest{ServiceName:itemName,Image:image,Anno:annos,Namespace:"default",InstanceNum:1}
	deploymentsClient := getClient().AppsV1beta1().Deployments(request.Namespace)

	_, err := deploymentsClient.Update(service.GetDeployment(request))
	if err!=nil{
		panic(err)
	}
}