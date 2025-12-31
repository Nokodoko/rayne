package pl

import (
	"fmt"
	"net/http"
)

func ImageRotation(w http.ResponseWriter, r *http.Request, name string) (int, any) {
	switch name {
	case "stop":
		_, err := serviceActions("stop ", name)
		if err != nil {
			fmt.Println(err)
			return http.StatusInternalServerError, nil
		}
	case "pull":
		_, err := serviceActions("pull", IMAGE)
		if err != nil {
			fmt.Println(err)
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error"))
		return http.StatusInternalServerError, nil
	case "remove":
		_, err := serviceActions("remove", IMAGE)
		if err != nil {
			fmt.Println(err)
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error"))
		return http.StatusInternalServerError, nil
	case "pp":
		_, err := serviceActions("pp", IMAGE)
		if err != nil {
			fmt.Println(err)
		}
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error"))
		return http.StatusInternalServerError, nil

	case "restart":
		_, err := serviceActions("restart", name)
		if err != nil {
			fmt.Println(err)
		}
		return http.StatusInternalServerError, nil
	}
	return http.StatusOK, nil
}
