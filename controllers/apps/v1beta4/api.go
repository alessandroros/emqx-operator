/*
Copyright 2021.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta4

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/emqx/emqx-operator/internal/apiclient"
	innerErr "github.com/emqx/emqx-operator/internal/errors"

	emperror "emperror.dev/errors"
	appsv1beta4 "github.com/emqx/emqx-operator/apis/apps/v1beta4"
	"github.com/tidwall/gjson"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type requestAPI struct {
	Username string
	Password string
	Port     string
	client.Client
	*apiclient.APIClient
}

func newRequestAPI(client client.Client, apiClient *apiclient.APIClient, instance appsv1beta4.Emqx) (*requestAPI, error) {
	username, password, err := getBootstrapUser(client, instance)
	if err != nil {
		return nil, err
	}

	return &requestAPI{
		Username:  username,
		Password:  password,
		Port:      "8081",
		Client:    client,
		APIClient: apiClient,
	}, nil
}

func getBootstrapUser(client client.Client, instance appsv1beta4.Emqx) (username, password string, err error) {
	bootstrapUser := &corev1.Secret{}
	if err = client.Get(context.Background(), types.NamespacedName{
		Namespace: instance.GetNamespace(),
		Name:      instance.GetName() + "-bootstrap-user",
	}, bootstrapUser); err != nil {
		err = emperror.Wrap(err, "get secret failed")
		return
	}

	if data, ok := bootstrapUser.Data["bootstrap_user"]; ok {
		users := strings.Split(string(data), "\n")
		for _, user := range users {
			index := strings.Index(user, ":")
			if index > 0 && user[:index] == defUsername {
				username = user[:index]
				password = user[index+1:]
				return
			}
		}
	}

	err = emperror.Errorf("the secret does not contain the bootstrap_user")
	return
}

func (r *requestAPI) requestAPI(instance appsv1beta4.Emqx, method, path string, body []byte) (*http.Response, []byte, error) {
	list, err := getInClusterStatefulSets(r.Client, instance)
	if path == "api/v4/nodes" && instance.GetStatus().GetEmqxNodes() == nil {
		list, err = getAllStatefulSet(r.Client, instance)
	}
	if err != nil {
		return nil, nil, err
	}
	sts := list[len(list)-1]
	podMap, err := getPodMap(r.Client, instance, []*appsv1.StatefulSet{sts})
	if err != nil {
		return nil, nil, err
	}
	if len(podMap[sts.UID]) == 0 {
		return nil, nil, innerErr.ErrPodNotReady
	}

	for _, pod := range podMap[sts.UID] {
		for _, container := range pod.Status.ContainerStatuses {
			if container.Name == EmqxContainerName {
				if container.Ready {
					return r.APIClient.RequestAPI(pod, r.Username, r.Password, r.Port, method, path, body)
				}
			}
		}
	}
	return nil, nil, innerErr.ErrPodNotReady
}

// Node
func (r *requestAPI) getNodeStatusesByAPI(instance appsv1beta4.Emqx) ([]appsv1beta4.EmqxNode, error) {
	_, body, err := r.requestAPI(instance, "GET", "api/v4/nodes", nil)
	if err != nil {
		return nil, err
	}

	emqxNodes := []appsv1beta4.EmqxNode{}
	data := gjson.GetBytes(body, "data")
	if err := json.Unmarshal([]byte(data.Raw), &emqxNodes); err != nil {
		return nil, emperror.Wrap(err, "failed to unmarshal node statuses")
	}
	return emqxNodes, nil
}

func (r *requestAPI) getListenerPortsByAPI(instance appsv1beta4.Emqx) ([]corev1.ServicePort, error) {
	type emqxListener struct {
		Protocol string `json:"protocol"`
		ListenOn string `json:"listen_on"`
	}

	type emqxListeners struct {
		Node      string         `json:"node"`
		Listeners []emqxListener `json:"listeners"`
	}

	intersection := func(listeners1 []emqxListener, listeners2 []emqxListener) []emqxListener {
		hSection := map[string]struct{}{}
		ans := make([]emqxListener, 0)
		for _, listener := range listeners1 {
			hSection[listener.ListenOn] = struct{}{}
		}
		for _, listener := range listeners2 {
			_, ok := hSection[listener.ListenOn]
			if ok {
				ans = append(ans, listener)
				delete(hSection, listener.ListenOn)
			}
		}
		return ans
	}

	_, body, err := r.requestAPI(instance, "GET", "api/v4/listeners", nil)
	if err != nil {
		return nil, err
	}

	listenerList := []emqxListeners{}
	data := gjson.GetBytes(body, "data")
	if err := json.Unmarshal([]byte(data.Raw), &listenerList); err != nil {
		return nil, emperror.Wrap(err, "failed to unmarshal node statuses")
	}

	var listeners []emqxListener
	if len(listenerList) == 1 {
		listeners = listenerList[0].Listeners
	} else {
		for i := 0; i < len(listenerList)-1; i++ {
			listeners = intersection(listenerList[i].Listeners, listenerList[i+1].Listeners)
		}
	}

	ports := []corev1.ServicePort{}
	for _, l := range listeners {
		var name string
		var protocol corev1.Protocol
		var strPort string
		var intPort int

		compile := regexp.MustCompile(".*(udp|dtls|sn).*")
		if compile.MatchString(l.Protocol) {
			protocol = corev1.ProtocolUDP
		} else {
			protocol = corev1.ProtocolTCP
		}

		if strings.Contains(l.ListenOn, ":") {
			_, strPort, err = net.SplitHostPort(l.ListenOn)
			if err != nil {
				strPort = l.ListenOn
			}
		} else {
			strPort = l.ListenOn
		}
		intPort, _ = strconv.Atoi(strPort)

		// Get name by protocol and port from API
		// protocol maybe like mqtt:wss:8084
		// protocol maybe like mqtt:tcp
		// We had to do something with the "protocol" to make it conform to the kubernetes service port name specification
		name = regexp.MustCompile(`:[\d]+`).ReplaceAllString(l.Protocol, "")
		name = strings.ReplaceAll(name, ":", "-")
		name = fmt.Sprintf("%s-%s", name, strPort)

		ports = append(ports, corev1.ServicePort{
			Name:       name,
			Protocol:   protocol,
			Port:       int32(intPort),
			TargetPort: intstr.FromInt(intPort),
		})
	}
	return ports, nil
}

// Evacuation
func (r *requestAPI) getEvacuationStatusByAPI(instance appsv1beta4.Emqx) ([]appsv1beta4.EmqxEvacuationStatus, error) {
	_, body, err := r.requestAPI(instance, "GET", "api/v4/load_rebalance/global_status", nil)
	if err != nil {
		return nil, err
	}

	evacuationStatuses := []appsv1beta4.EmqxEvacuationStatus{}
	data := gjson.GetBytes(body, "evacuations")
	if err := json.Unmarshal([]byte(data.Raw), &evacuationStatuses); err != nil {
		return nil, emperror.Wrap(err, "failed to unmarshal node statuses")
	}
	return evacuationStatuses, nil
}

func (r *requestAPI) startEvacuateNodeByAPI(instance appsv1beta4.Emqx, migrateToPods []*corev1.Pod, nodeName string) error {
	enterprise, ok := instance.(*appsv1beta4.EmqxEnterprise)
	if !ok {
		return emperror.New("failed to evacuate node, only support emqx enterprise")
	}

	migrateTo := []string{}
	for _, pod := range migrateToPods {
		emqxNodeName := getEmqxNodeName(instance, pod)
		migrateTo = append(migrateTo, emqxNodeName)
	}

	body := map[string]interface{}{
		"conn_evict_rate": enterprise.Spec.EmqxBlueGreenUpdate.EvacuationStrategy.ConnEvictRate,
		"sess_evict_rate": enterprise.Spec.EmqxBlueGreenUpdate.EvacuationStrategy.SessEvictRate,
		"migrate_to":      migrateTo,
	}
	if enterprise.Spec.EmqxBlueGreenUpdate.EvacuationStrategy.WaitTakeover > 0 {
		body["wait_takeover"] = enterprise.Spec.EmqxBlueGreenUpdate.EvacuationStrategy.WaitTakeover
	}

	b, err := json.Marshal(body)
	if err != nil {
		return emperror.Wrap(err, "marshal body failed")
	}

	_, _, err = r.requestAPI(instance, "POST", "api/v4/load_rebalance/"+nodeName+"/evacuation/start", b)
	return err
}

// Plugin
func (r *requestAPI) loadPluginByAPI(emqx appsv1beta4.Emqx, nodeName, pluginName, reloadOrUnload string) error {
	resp, _, err := r.requestAPI(emqx, "PUT", fmt.Sprintf("api/v4/nodes/%s/plugins/%s/%s", nodeName, pluginName, reloadOrUnload), nil)
	if err != nil {
		return err
	}
	if resp.StatusCode != 200 {
		return emperror.Errorf("request api failed: %s", resp.Status)
	}
	return nil
}

func (r *requestAPI) getPluginsByAPI(emqx appsv1beta4.Emqx) ([]pluginListByAPIReturn, error) {
	var data []pluginListByAPIReturn
	resp, body, err := r.requestAPI(emqx, "GET", "api/v4/plugins", nil)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != 200 {
		return nil, emperror.Errorf("request api failed: %s", resp.Status)
	}

	err = json.Unmarshal([]byte(gjson.GetBytes(body, "data").String()), &data)
	if err != nil {
		return nil, err
	}

	return data, nil
}
