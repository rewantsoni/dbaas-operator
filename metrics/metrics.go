package metrics

import (
	dbaasv1alpha1 "github.com/RHEcosystemAppEng/dbaas-operator/api/v1alpha1"
	"github.com/go-logr/logr"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"net/http"
)

var (
	PlatformStatus = promauto.NewGaugeVec(prometheus.GaugeOpts{
		Name: "dbaas_platform_status",
		Help: "status for the installation of components and provider operators",
	},
		[]string{
			"platform",
			"status",
		})
)

// SetPlatformStatusMetric exposes dbaas_platform_status metric for each platform
func SetPlatformStatusMetric(log logr.Logger, platformName dbaasv1alpha1.PlatformsName, status dbaasv1alpha1.PlatformsInstlnStatus) {
	log.Info("setting metrics", string(platformName), string(status))
	switch status {
	case dbaasv1alpha1.ResultFailed:
		PlatformStatus.With(prometheus.Labels{"platform": string(platformName), "status": string(status)}).Set(0)
	case dbaasv1alpha1.ResultSuccess:
		log.Info("setting metrics", string(platformName), string(status))
		PlatformStatus.Delete(prometheus.Labels{"platform": string(platformName), "status": string(dbaasv1alpha1.ResultInProgress)})
		PlatformStatus.Delete(prometheus.Labels{"platform": string(platformName), "status": string(dbaasv1alpha1.ResultFailed)})
		PlatformStatus.With(prometheus.Labels{"platform": string(platformName), "status": string(status)}).Set(1)
	case dbaasv1alpha1.ResultInProgress:
		PlatformStatus.With(prometheus.Labels{"platform": string(platformName), "status": string(status)}).Set(2)
	}

}

// CleanPlatformStatusMetric delete the dbaas_platform_status metric for each platform
func CleanPlatformStatusMetric(log logr.Logger, platformName dbaasv1alpha1.PlatformsName, status dbaasv1alpha1.PlatformsInstlnStatus) {
	if status == dbaasv1alpha1.ResultSuccess {
		PlatformStatus.Delete(prometheus.Labels{"platform": string(platformName), "status": string(dbaasv1alpha1.ResultSuccess)})
	}
}

func Metrics() {
	http.Handle("/metrics", promhttp.Handler())
	http.ListenAndServe(":2112", nil)
}
