package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"migrated-app/internal/model"
	"migrated-app/internal/service"
)

// Plain-text success messages returned by the source controller. They are
// client-visible and reproduced verbatim (including capitalisation).
const (
	msgUserSaved   = "User data saved successfully!"
	msgUserDeleted = "user data deleted Successfully"
)

// textPlainUTF8 is the Content-Type Spring's StringHttpMessageConverter uses
// for String return values.
const textPlainUTF8 = "text/plain;charset=UTF-8"

// UserHandler exposes the User CRUD endpoints. It replaces the Spring
// @RestController UserController; the UserService dependency is injected
// explicitly through NewUserHandler instead of @Autowired field injection.
type UserHandler struct {
	svc service.UserService
}

// NewUserHandler returns a UserHandler backed by svc.
func NewUserHandler(svc service.UserService) (*UserHandler, error) {
	if svc == nil {
		return nil, errors.New("httpapi: nil user service")
	}
	return &UserHandler{svc: svc}, nil
}

// Routes returns an http.Handler with every User endpoint registered on a
// Go 1.22 ServeMux. The mux is wrapped so that it behaves like Spring MVC:
//   - a single trailing slash is ignored (/get_user_data/ == /get_user_data);
//   - a known path with the wrong method yields 405 with an Allow header and
//     an empty body (ResponseEntityExceptionHandler behaviour);
//   - an unknown path yields Spring's default 404 error JSON.
func (h *UserHandler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /save_user_data", h.SaveUser)
	mux.HandleFunc("GET /get_user_data", h.FetchUserList)
	mux.HandleFunc("GET /get_user_data/{id}", h.FetchUserByID)
	mux.HandleFunc("DELETE /delete_user_data/{id}", h.DeleteUser)
	mux.HandleFunc("PUT /update_user_data/{id}", h.UpdateUser)
	// Source mapping was "get_user_name/name/{name}"; Spring prepends the
	// missing leading slash.
	mux.HandleFunc("GET /get_user_name/name/{name}", h.GetUserNameByName)
	return springCompat(mux)
}

// SaveUser handles POST /save_user_data. The body is decoded into a User and
// validated (@Valid); malformed JSON or a validation failure yields an empty
// 400, a persistence failure yields 500.
func (h *UserHandler) SaveUser(w http.ResponseWriter, r *http.Request) {
	slog.Info("inside the saveUser of UserController ")

	u, ok := decodeUser(r)
	if !ok {
		writeEmpty(w, http.StatusBadRequest)
		return
	}
	if err := u.Validate(); err != nil {
		// MethodArgumentNotValidException → ResponseEntityExceptionHandler
		// answers 400 with an empty body.
		writeEmpty(w, http.StatusBadRequest)
		return
	}
	if _, err := h.svc.SaveUser(r.Context(), u); err != nil {
		WriteError(w, r, err)
		return
	}
	writeText(w, http.StatusOK, msgUserSaved)
}

// FetchUserList handles GET /get_user_data and returns all users as a JSON
// array.
func (h *UserHandler) FetchUserList(w http.ResponseWriter, r *http.Request) {
	slog.Info("inside the fetchUserList of UserController ")

	users, err := h.svc.FetchUserList(r.Context())
	if err != nil {
		WriteError(w, r, err)
		return
	}
	if users == nil {
		users = make([]*model.User, 0)
	}
	writeJSON(w, http.StatusOK, users)
}

// FetchUserByID handles GET /get_user_data/{id}. A non-int32 id yields an
// empty 400; a missing user yields 404 with an ErrorMessage body.
func (h *UserHandler) FetchUserByID(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeEmpty(w, http.StatusBadRequest)
		return
	}
	u, err := h.svc.FetchUserByID(r.Context(), id)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	writeUserOrEmpty(w, u)
}

// DeleteUser handles DELETE /delete_user_data/{id}. Deleting a nonexistent id
// is an error (500), mirroring Spring Data's EmptyResultDataAccessException.
func (h *UserHandler) DeleteUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeEmpty(w, http.StatusBadRequest)
		return
	}
	if err := h.svc.DeleteUser(r.Context(), id); err != nil {
		WriteError(w, r, err)
		return
	}
	writeText(w, http.StatusOK, msgUserDeleted)
}

// UpdateUser handles PUT /update_user_data/{id}. The body is NOT validated
// (the source has no @Valid here). The request body is echoed back with its
// id set to the path id.
func (h *UserHandler) UpdateUser(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(r)
	if !ok {
		writeEmpty(w, http.StatusBadRequest)
		return
	}
	u, ok := decodeUser(r)
	if !ok {
		writeEmpty(w, http.StatusBadRequest)
		return
	}
	// Snapshot the request body: the Java controller returned the object it
	// received (after service.setId(id)), not the merged entity, so any id
	// the repository may assign on insert must not leak into the response.
	echo := *u
	echo.ID = id
	if err := h.svc.UpdateUser(r.Context(), id, u); err != nil {
		WriteError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, &echo)
}

// GetUserNameByName handles GET /get_user_name/name/{name}. When no user
// matches, the source returned null, which Spring renders as 200 with an
// empty body. Multiple matches yield 500.
func (h *UserHandler) GetUserNameByName(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("name")
	u, found, err := h.svc.GetUserNameByName(r.Context(), name)
	if err != nil {
		WriteError(w, r, err)
		return
	}
	if !found {
		u = nil
	}
	writeUserOrEmpty(w, u)
}

// decodeUser decodes the request body into a User. Unknown fields are
// allowed (Jackson default under Spring Boot). An empty body, malformed JSON
// or a JSON null body reports false (HttpMessageNotReadableException → 400).
func decodeUser(r *http.Request) (*model.User, bool) {
	if r.Body == nil {
		return nil, false
	}
	var u *model.User
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		return nil, false
	}
	if u == nil {
		return nil, false
	}
	return u, true
}

// pathID parses the {id} path variable as a 32-bit signed integer, matching
// Java's int @PathVariable conversion.
func pathID(r *http.Request) (int, bool) {
	n, err := strconv.ParseInt(r.PathValue("id"), 10, 32)
	if err != nil {
		return 0, false
	}
	return int(n), true
}

// writeUserOrEmpty writes u as JSON, or a 200 with an empty body when u is
// nil (Spring's rendering of a null @ResponseBody).
func writeUserOrEmpty(w http.ResponseWriter, u *model.User) {
	if u == nil {
		w.WriteHeader(http.StatusOK)
		return
	}
	writeJSON(w, http.StatusOK, u)
}

// writeText writes a plain-text body with the given status.
func writeText(w http.ResponseWriter, status int, body string) {
	w.Header().Set("Content-Type", textPlainUTF8)
	w.WriteHeader(status)
	if _, err := w.Write([]byte(body)); err != nil {
		slog.Error("failed to write text response", "error", err)
	}
}

// springCompat wraps mux with Spring MVC compatible trailing-slash matching
// and 404/405 handling for requests that match no registered pattern.
func springCompat(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if p := r.URL.Path; len(p) > 1 && strings.HasSuffix(p, "/") {
			r2 := r.Clone(r.Context())
			r2.URL.Path = strings.TrimSuffix(p, "/")
			if r2.URL.RawPath != "" {
				r2.URL.RawPath = strings.TrimSuffix(r2.URL.RawPath, "/")
			}
			r = r2
		}

		if _, pattern := mux.Handler(r); pattern != "" {
			mux.ServeHTTP(w, r)
			return
		}

		// No pattern matched: the mux will answer 404 or 405 itself. Intercept
		// its plain-text reply and replace it with Spring's shapes.
		iw := &interceptWriter{ResponseWriter: w, r: r}
		mux.ServeHTTP(iw, r)
	})
}

// interceptWriter rewrites ServeMux's built-in 404/405 replies.
type interceptWriter struct {
	http.ResponseWriter
	r       *http.Request
	handled bool
}

// WriteHeader replaces ServeMux's 404 with Spring's error JSON and its 405
// with an empty body (keeping the Allow header).
func (iw *interceptWriter) WriteHeader(status int) {
	if iw.handled {
		return
	}
	iw.handled = true
	h := iw.ResponseWriter.Header()
	h.Del("Content-Type")
	h.Del("X-Content-Type-Options")
	switch status {
	case http.StatusNotFound:
		NotFoundRoute(iw.ResponseWriter, iw.r)
	case http.StatusMethodNotAllowed:
		writeEmpty(iw.ResponseWriter, http.StatusMethodNotAllowed)
	default:
		iw.ResponseWriter.WriteHeader(status)
	}
}

// Write discards ServeMux's plain-text body once the reply was rewritten.
func (iw *interceptWriter) Write(b []byte) (int, error) {
	if !iw.handled {
		iw.WriteHeader(http.StatusOK)
	}
	return len(b), nil
}
