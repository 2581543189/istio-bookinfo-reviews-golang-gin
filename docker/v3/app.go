package main

import (
	"encoding/json"
	"github.com/gin-gonic/gin"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)
import "os"

// 收集环境变量
var ratings_enabled, _ = strconv.ParseBool(os.Getenv("ENABLE_RATINGS"))
var services_domain = "." + os.Getenv("SERVICES_DOMAIN")
var ratings_hostname = os.Getenv("RATINGS_HOSTNAME")
var ratings_service = ""
var star_color = os.Getenv("STAR_COLOR")
var headers_to_propagate = []string{
	// All applications should propagate x-request-id. This header is
	// included in access log statements and is used for consistent trace
	// sampling and log sampling decisions in Istio.
	"x-request-id",

	// Lightstep tracing header. Propagate this if you use lightstep tracing
	// in Istio (see
	// https://istio.io/latest/docs/tasks/observability/distributed-tracing/lightstep/)
	// Note: this should probably be changed to use B3 or W3C TRACE_CONTEXT.
	// Lightstep recommends using B3 or TRACE_CONTEXT and most application
	// libraries from lightstep do not support x-ot-span-context.
	"x-ot-span-context",

	// Datadog tracing header. Propagate these headers if you use Datadog
	// tracing.
	"x-datadog-trace-id",
	"x-datadog-parent-id",
	"x-datadog-sampling-priority",

	// W3C Trace Context. Compatible with OpenCensusAgent and Stackdriver Istio
	// configurations.
	"traceparent",
	"tracestate",

	// Cloud trace context. Compatible with OpenCensusAgent and Stackdriver Istio
	// configurations.
	"x-cloud-trace-context",

	// Grpc binary trace context. Compatible with OpenCensusAgent nad
	// Stackdriver Istio configurations.
	"grpc-trace-bin",

	// b3 trace headers. Compatible with Zipkin, OpenCensusAgent, and
	// Stackdriver Istio configurations. Commented out since they are
	// propagated by the OpenTracing tracer above.
	"x-b3-traceid",
	"x-b3-spanid",
	"x-b3-parentspanid",
	"x-b3-sampled",
	"x-b3-flags",

	// Application-specific headers to forward.
	"end-user",
	"user-agent",
}

type RatingsResp struct {
	Ratings *Ratings `json:"ratings"`
}

type Ratings struct {
	Reviewer1 int `json:"Reviewer1"`
	Reviewer2 int `json:"Reviewer2"`
}

// 初始化变量
func init() {
	if len(services_domain) == 1 {
		services_domain = ""
	}
	if len(ratings_hostname) == 0 {
		ratings_hostname = "ratings"
	}
	ratings_service = "http://" + ratings_hostname + services_domain + ":9080/ratings"
	if len(star_color) == 0 {
		star_color = "black"
	}
}

func main() {
	r := gin.Default()

	//
	r.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "hi",
		})
	})

	//health
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status": "Reviews is healthy",
		})
	})
	//reviews
	r.GET("/reviews/:productId", func(c *gin.Context) {
		productId, _ := strconv.Atoi(c.Param("productId"))
		rr := &RatingsResp{&Ratings{-1, -1}}
		if ratings_enabled {
			ratingsResponse := getRatings(productId, c)
			if len(ratingsResponse) > 0 {
				json.Unmarshal(ratingsResponse, rr)
			}
		}
		resp := getJsonResponse(productId,rr.Ratings.Reviewer1,rr.Ratings.Reviewer2)
		c.String(200,resp)
	})
	r.Run(":9080")
}

func getRatings(productId int, c *gin.Context) []byte {

	resp := []byte("")
	// 设置超时时间
	timeout := 2500 * time.Millisecond
	if star_color == "black" {
		timeout = 10000 * time.Millisecond
	}
	client := &http.Client{Timeout: timeout}

	// 构造req
	url := ratings_service + "/" + strconv.FormatInt(int64(productId), 10)
	reqest, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return resp
	}
	for i := range headers_to_propagate {
		reqest.Header.Add("Cookie", c.Request.Header.Get(headers_to_propagate[i]))
	}

	//处理返回结果
	response, _ := client.Do(reqest)
	defer response.Body.Close()

	if err != nil || response.StatusCode != http.StatusOK {
		return resp
	}

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return resp
	}

	return body
}

func getJsonResponse(productId, starsReviewer1, starsReviewer2 int) string {
	result := "{"
	result += "\"id\": \"" + strconv.FormatInt(int64(productId), 10) + "\","
	result += "\"reviews\": ["

	// reviewer 1:
	result += "{"
	result += "  \"reviewer\": \"Reviewer1\","
	result += "  \"text\": \"An extremely entertaining play by Shakespeare. The slapstick humour is refreshing!\""
	if ratings_enabled {
		if starsReviewer1 != -1 {
			result += ", \"rating\": {\"stars\": " + strconv.FormatInt(int64(starsReviewer1), 10) + ", \"color\": \"" + star_color + "\"}"
		} else {
			result += ", \"rating\": {\"error\": \"Ratings service is currently unavailable\"}"
		}
	}
	result += "},"

	// reviewer 2:
	result += "{"
	result += "  \"reviewer\": \"Reviewer2\","
	result += "  \"text\": \"Absolutely fun and entertaining. The play lacks thematic depth when compared to other plays by Shakespeare.\""
	if ratings_enabled {
		if starsReviewer2 != -1 {
			result += ", \"rating\": {\"stars\": " + strconv.FormatInt(int64(starsReviewer2), 10) + ", \"color\": \"" + star_color + "\"}"
		} else {
			result += ", \"rating\": {\"error\": \"Ratings service is currently unavailable\"}"
		}
	}
	result += "}"
	result += "]"
	result += "}"
	return result
}
