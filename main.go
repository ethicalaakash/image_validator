package main

import (
	"encoding/json"
	"flag"
	"fmt"
	validator "image_validator/pkg"
	"log"
	"net/http"
	"os"
	"strconv"

	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
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
	http.HandleFunc("/validate", validate)
	logger.Println("Starting server..")
	log.Fatal(http.ListenAndServeTLS(":"+strconv.Itoa(parameters.port), parameters.certFile, parameters.keyFile, nil))

}

func validate(w http.ResponseWriter, r *http.Request) {
	logger.Printf("Received message on validater")
	deserializer := codecs.UniversalDeserializer()

	admissionReviewRequest, err := validator.AdmissionReviewFromRequest(r, deserializer)
	if err != nil {
		msg := fmt.Sprintf("error getting admission review from request: %v", err)
		logger.Printf(msg)
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(msg))
		return
	}

	kind := admissionReviewRequest.Request.Kind.Kind
	logger.Printf("AdmissionReview for Kind=%v", kind)
	rawRequest := admissionReviewRequest.Request.Object.Raw
	if kind == "Deployment" {
		admissionResponse, err := validator.ValidateDeployment(rawRequest, deserializer)
		if err != nil {
			msg := fmt.Sprintf("error decoding raw deployment: %v", err)
			logger.Printf(msg)
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(msg))
			return
		}
		resp, err := reviewResponse(admissionResponse, admissionReviewRequest)
		if err != nil {
			msg := fmt.Sprintf("error marshalling response json: %v", err)
			logger.Printf(msg)
			w.WriteHeader(500)
			w.Write([]byte(msg))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(resp)
		return
	} else if kind == "Pod" {
		admissionResponse, err := validator.ValidatePod(rawRequest, deserializer)
		if err != nil {
			msg := fmt.Sprintf("error decoding raw deployment: %v", err)
			logger.Printf(msg)
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(msg))
			return
		}
		resp, err := reviewResponse(admissionResponse, admissionReviewRequest)
		if err != nil {
			msg := fmt.Sprintf("error marshalling response json: %v", err)
			logger.Printf(msg)
			w.WriteHeader(500)
			w.Write([]byte(msg))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(resp)
		return
	}
	msg := fmt.Sprintf("Got kind %v. Expected Deployment or pod", kind)
	logger.Printf(msg)
	w.WriteHeader(http.StatusBadRequest)
	w.Write([]byte(msg))
}

func reviewResponse(admissionResponse *admissionv1.AdmissionResponse, admissionReviewRequest *admissionv1.AdmissionReview) ([]byte, error) {
	var admissionReviewResponse admissionv1.AdmissionReview
	admissionReviewResponse.Response = admissionResponse
	admissionReviewResponse.SetGroupVersionKind(admissionReviewRequest.GroupVersionKind())
	admissionReviewResponse.Response.UID = admissionReviewRequest.Request.UID
	resp, err := json.Marshal(admissionReviewResponse)
	if err != nil {
		return nil, err
	}
	return resp, nil
}
