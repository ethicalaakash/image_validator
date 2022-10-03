package validator

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func ImageChecker(containers []corev1.Container) *admissionv1.AdmissionResponse {
	var logger = log.New(os.Stdout, "http:", log.LstdFlags)
	admissionResponse := &admissionv1.AdmissionResponse{}
	for i := range containers {
		containerImage := containers[i].Image
		logger.Printf("validating Image: %v", containerImage)
		if strings.Contains(containerImage, "/") {
			registry := strings.Split(containerImage, "/")
			if registry[0] == "quay.io" {
				logger.Printf("this image will be depricated soon")
				admissionResponse.Allowed = true
				admissionResponse.Warnings = []string{"quay is will be depriated soon, please move to ecr"}
			} else if strings.Contains(registry[0], "ecr") {
				logger.Printf("valid image")
				admissionResponse.Allowed = true
			} else {
				logger.Printf("%v is not a valid registry", registry[0])
				admissionResponse.Allowed = false
				admissionResponse.Result = &metav1.Status{
					Message: containerImage + " Not a valid Image",
				}
				break
			}
		} else {
			logger.Printf("%v is not a valid image", containerImage)
			admissionResponse.Allowed = false
			admissionResponse.Result = &metav1.Status{
				Message: containerImage + " Not a valid Image",
			}
			break
		}
	}
	return admissionResponse
}

func AdmissionReviewFromRequest(r *http.Request, deserializer runtime.Decoder) (*admissionv1.AdmissionReview, error) {
	// validate content type
	if r.Header.Get("Content-Type") != "application/json" {
		return nil, fmt.Errorf("expected application/json content-type")
	}

	var body []byte
	if r.Body != nil {
		requestData, err := io.ReadAll(r.Body)
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

func ValidateDeployment(rawRequest []byte, deserializer runtime.Decoder) (*admissionv1.AdmissionResponse, error) {
	deployment := appsv1.Deployment{}

	if _, _, err := deserializer.Decode(rawRequest, nil, &deployment); err != nil {
		return nil, err
	}
	containers := deployment.Spec.Template.Spec.Containers
	admissionResponse := ImageChecker(containers)
	return admissionResponse, nil
}

func ValidatePod(rawRequest []byte, deserializer runtime.Decoder) (*admissionv1.AdmissionResponse, error) {
	pod := corev1.Pod{}
	if _, _, err := deserializer.Decode(rawRequest, nil, &pod); err != nil {
		return nil, err
	}
	containers := pod.Spec.Containers
	admissionResponse := ImageChecker(containers)
	return admissionResponse, nil
}
