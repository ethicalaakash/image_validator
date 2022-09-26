package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	// "k8s.io/utils/strings"
)

type ServerParameters struct {
	port     int    // webhook server port
	certFile string // path for cert for https
	keyFile  string // path to private key for cert
}

var (
	logger = log.New(os.Stdout, "http:", log.LstdFlags)
	codecs = serializer.NewCodecFactory(runtime.NewScheme())
)
var parameters ServerParameters

func main() {
	flag.IntVar(&parameters.port, "port", 8443, "Webhook server port.")
	flag.StringVar(&parameters.certFile, "tlsCertFile", "/etc/webhook/certs/tls.crt", "File containing the x509 Certificate for HTTPS.")
	flag.StringVar(&parameters.keyFile, "tlsKeyFile", "/etc/webhook/certs/tls.key", "File containing the x509 private key to --tlsCertFile.")
	flag.Parse()
	// http.HandleFunc("/", HandleRoot)
	http.HandleFunc("/validate", validatePod)
	log.Fatal(http.ListenAndServeTLS(":"+strconv.Itoa(parameters.port), parameters.certFile, parameters.keyFile, nil))
}

func validatePod(w http.ResponseWriter, r *http.Request) {
	logger.Printf("Received message on validater")
	deserializer := codecs.UniversalDeserializer()

	admissionReviewRequest, err := admissionReviewFromRequest(r, deserializer)
	if err != nil {
		msg := fmt.Sprintf("error getting admission review from request: %v", err)
		logger.Printf(msg)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(msg))
		return
	}

	rawRequest := admissionReviewRequest.Request.Object.Raw
	pod := corev1.Pod{}
	if _, _, err := deserializer.Decode(rawRequest, nil, &pod); err != nil {
		msg := fmt.Sprintf("error decoding raw pod: %v", err)
		logger.Printf(msg)
		w.WriteHeader(500)
		w.Write([]byte(msg))
		return
	}

	admissionResponse := &admissionv1.AdmissionResponse{}
	admissionResponse.Allowed = false
	for i := range pod.Spec.Containers {
		containerImage := pod.Spec.Containers[i].Image
		logger.Printf(containerImage)
		if strings.Contains(containerImage, "/") {
			registry := strings.Split(containerImage, "/")
			if registry[0] == "quay.io" {
				admissionResponse.Allowed = true
				admissionResponse.Warnings = []string{"quay is will be depriated soon, please move to ecr"}
			} else if strings.Contains(registry[0], "ecr") {
				admissionResponse.Allowed = true
			} else {
				admissionResponse.Allowed = false
				admissionResponse.Result = &metav1.Status{
					Message: containerImage + " Not a valid Image",
				}
			}
		} else {
			admissionResponse.Allowed = false
			admissionResponse.Result = &metav1.Status{
				Message: containerImage + " Not a valid Image",
			}
		}
	}
	var admissionReviewResponse admissionv1.AdmissionReview
	admissionReviewResponse.Response = admissionResponse
	admissionReviewResponse.SetGroupVersionKind(admissionReviewRequest.GroupVersionKind())
	admissionReviewResponse.Response.UID = admissionReviewRequest.Request.UID
	resp, err := json.Marshal(admissionReviewResponse)
	if err != nil {
		msg := fmt.Sprintf("error marshalling response json: %v", err)
		logger.Printf(msg)
		w.WriteHeader(500)
		w.Write([]byte(msg))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(resp)
}

func admissionReviewFromRequest(r *http.Request, deserializer runtime.Decoder) (*admissionv1.AdmissionReview, error) {
	// validate content type
	if r.Header.Get("Content-Type") != "application/json" {
		return nil, fmt.Errorf("expected application/json content-type")
	}

	var body []byte
	if r.Body != nil {
		requestData, err := ioutil.ReadAll(r.Body)
		if err != nil {
			return nil, err
		}
		body = requestData
	}
	admissionReviewRequest := &admissionv1.AdmissionReview{}
	if _, _, err := deserializer.Decode(body, nil, admissionReviewRequest); err != nil {
		return nil, err
	}
	return admissionReviewRequest, nil
}
