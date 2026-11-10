package a

import "net/http"

func handler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", http.StatusMethodNotAllowed) // want `http.Error does not stop the handler`
		w.WriteHeader(http.StatusOK)
	}
}

func ok(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func sw(w http.ResponseWriter, r *http.Request, kind int) {
	switch kind {
	case 1:
		http.Error(w, "bad", http.StatusBadRequest) // want `http.Error does not stop the handler`
		w.WriteHeader(http.StatusOK)
	}
}

// A statement that writes somewhere else, such as a log, does not write the
// response a second time.
func logs(w http.ResponseWriter, r *http.Request, log func(...any)) {
	if r.Method != http.MethodGet {
		http.Error(w, "method", http.StatusMethodNotAllowed)
		log("method rejected")
		return
	}
}
