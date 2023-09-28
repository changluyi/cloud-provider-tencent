package tencentcloud

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"strings"
	"time"

	clb "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/clb/v20180317"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common"
	"github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/common/profile"
	cvm "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/cvm/v20170312"
	tke "github.com/tencentcloud/tencentcloud-sdk-go/tencentcloud/tke/v20180525"
	"github.com/yichanglu/cloud-provider-tencent/pkg/cache"
	cloudProvider "k8s.io/cloud-provider"
	"k8s.io/klog/v2"

	"k8s.io/client-go/kubernetes"
)

const (
	providerName = "tencentcloud"
	tkeName      = "tke"
	TTLTime      = 60 * time.Second
)

type TxCloudConfig struct {
	Region            string `json:"region"`
	VpcId             string `json:"vpc_id"`
	CLBNamePrefix     string `json:"clb_name_prefix"`
	TagKey            string `json:"tag_key"`
	SecretId          string `json:"secret_id"`
	SecretKey         string `json:"secret_key"`
	ClusterRouteTable string `json:"cluster_route_table"`
	EndPoint          string `json:"endpoint"`
}

type Cloud struct {
	txConfig   TxCloudConfig
	kubeClient kubernetes.Interface
	cvm        *cvm.Client
	tke        *tke.Client
	clb        *clb.Client
	cache      *cache.TTLCache
}

// NewCloud Cloud constructed function
func NewCloud(config io.Reader) (*Cloud, error) {
	var c TxCloudConfig
	if config != nil {
		cfg, err := ioutil.ReadAll(config)
		if err != nil {
			klog.V(3).Infof("tencentcloud.NewCloud: return: nil, %v\n", err)
			return nil, err
		}
		if err := json.Unmarshal(cfg, &c); err != nil {
			return nil, err
		}
	}

	if c.Region == "" {
		c.Region = os.Getenv("TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_REGION")
	}
	if c.VpcId == "" {
		c.VpcId = os.Getenv("TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_VPC_ID")
	}
	if c.CLBNamePrefix == "" {
		c.CLBNamePrefix = os.Getenv("TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_CLB_NAME_PREFIX")
		if c.CLBNamePrefix == "" {
			c.CLBNamePrefix = tkeName
		}
	}
	if c.TagKey == "" {
		c.TagKey = os.Getenv("TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_CLB_TAG_KEY")
		if c.TagKey == "" {
			c.TagKey = tkeName
		}
	}

	if c.SecretId == "" {
		c.SecretId = os.Getenv("TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_SECRET_ID")
	}
	if c.SecretKey == "" {
		c.SecretKey = os.Getenv("TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_SECRET_KEY")
	}

	if c.EndPoint == "" {
		c.EndPoint = os.Getenv("TENCENTCLOUD_CLOUD_CONTROLLER_MANAGER_CLUSTER_ENDPOINT")
	}

	if err := checkConfig(c); err != nil {
		klog.V(3).Infof("tencentcloud.NewCloud: return: nil, %v\n", err)
		return nil, err
	}

	return &Cloud{txConfig: c}, nil
}

// checkConfig check cloud config
func checkConfig(c TxCloudConfig) error {
	if strings.TrimSpace(c.Region) == "" {
		klog.Error("tencentcloud.checkConfig: 'Region' config is null\n")
		return errors.New("'Region' config is null")
	}
	if strings.TrimSpace(c.VpcId) == "" {
		klog.Error("tencentcloud.checkConfig: 'VpcId' config is null\n")
		return errors.New("'VpcId' config is null")
	}
	if strings.TrimSpace(c.TagKey) == "" {
		klog.Error("tencentcloud.checkConfig: 'TagKey' config is null\n")
		return errors.New("'TagKey' config is null")
	}
	if strings.TrimSpace(c.CLBNamePrefix) == "" {
		klog.Error("tencentcloud.checkConfig: 'CLBNamePrefix' config is null\n")
		return errors.New("'CLBNamePrefix' config is null")
	}
	if strings.TrimSpace(c.SecretId) == "" {
		klog.Error("tencentcloud.checkConfig: 'SecretId' config is null\n")
		return errors.New("'SecretId' config is null")
	}
	if strings.TrimSpace(c.SecretKey) == "" {
		klog.Error("tencentcloud.checkConfig: 'SecretKey' config is null\n")
		return errors.New("'SecretKey' config is null")
	}
	return nil
}

// init Initialize cloudProvider
func init() {
	cloudProvider.RegisterCloudProvider(providerName,
		func(config io.Reader) (cloudProvider.Interface, error) {
			return NewCloud(config)
		})
}

// Initialize provides the cloud with a kubernetes client builder and may spawn goroutines
// to perform housekeeping activities within the cloud provider.
func (cloud *Cloud) Initialize(clientBuilder cloudProvider.ControllerClientBuilder, stop <-chan struct{}) {
	cloud.kubeClient = clientBuilder.ClientOrDie("tencentcloud-cloud-provider")
	credential := common.NewCredential(
		cloud.txConfig.SecretId,
		cloud.txConfig.SecretKey,
	)

	cpfCVM := profile.NewClientProfile()
	cpfCVM.HttpProfile.ReqTimeout = 10
	cpfCVM.HttpProfile.Scheme = "HTTP"
	cpfCVM.HttpProfile.Endpoint = "cvm." + cloud.txConfig.EndPoint

	cvmClient, err := cvm.NewClient(credential, cloud.txConfig.Region, cpfCVM)
	if err != nil {
		klog.Warningf("tencentcloud.Initialize().cvm.NewClient An tencentcloud API error has returned, message=[%v])\n", err)
	}
	cloud.cvm = cvmClient

	cpfTKE := profile.NewClientProfile()
	cpfTKE.HttpProfile.ReqTimeout = 10
	cpfTKE.HttpProfile.Scheme = "HTTP"
	cpfTKE.HttpProfile.Endpoint = "tke." + cloud.txConfig.EndPoint
	tkeClient, err := tke.NewClient(credential, cloud.txConfig.Region, cpfTKE)
	if err != nil {
		klog.Warningf("tencentcloud.Initialize().tke.NewClient An tencentcloud API error has returned, message=[%v])\n", err)
	}
	cloud.tke = tkeClient

	cpfCLB := profile.NewClientProfile()
	cpfCLB.HttpProfile.ReqTimeout = 10
	cpfCLB.HttpProfile.Scheme = "HTTP"
	cpfCLB.HttpProfile.Endpoint = "clb." + cloud.txConfig.EndPoint
	clbClient, err := clb.NewClient(credential, cloud.txConfig.Region, cpfCLB)
	if err != nil {
		klog.Warningf("tencentcloud.Initialize().clb.NewClient An tencentcloud API error has returned, message=[%v])\n", err)
	}
	cloud.clb = clbClient

	cloud.cache = cache.NewTTLCache(TTLTime)
}

// LoadBalancer returns a balancer interface. Also returns true if the interface is supported, false otherwise.
func (cloud *Cloud) LoadBalancer() (cloudProvider.LoadBalancer, bool) {
	return cloud, true
}

// Instances returns an instances interface. Also returns true if the interface is supported, false otherwise.
func (cloud *Cloud) Instances() (cloudProvider.Instances, bool) {
	return cloud, true
}

// InstancesV2 is an implementation for instances and should only be implemented by external cloud providers.
// Don't support this feature for now.
func (cloud *Cloud) InstancesV2() (cloudProvider.InstancesV2, bool) {
	return nil, false
}

// Zones returns a zones interface. Also returns true if the interface is supported, false otherwise.
func (cloud *Cloud) Zones() (cloudProvider.Zones, bool) {
	return nil, false
}

// Clusters returns a clusters interface.  Also returns true if the interface is supported, false otherwise.
func (cloud *Cloud) Clusters() (cloudProvider.Clusters, bool) {
	return nil, false
}

// Routes returns a routes interface along with whether the interface is supported.
func (cloud *Cloud) Routes() (cloudProvider.Routes, bool) {
	return nil, false
}

// ProviderName returns the cloud provider ID.
func (cloud *Cloud) ProviderName() string {
	return providerName
}

// HasClusterID returns true if a ClusterID is required and set
func (cloud *Cloud) HasClusterID() bool {
	return false
}
