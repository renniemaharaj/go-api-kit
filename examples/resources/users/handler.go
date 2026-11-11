package users

import (
	"log"
	"net/http"

	"github.com/renniemaharaj/go-api-kit/kit"
)

// Routes registers optional grouping and policies directly on the kit router.
func (u *Users) Routes(router *kit.Router, middleware ...kit.Middleware) {
	router = router.With(middleware...)
	for _, route := range []struct {
		method string
		handle kit.HandlerFunc
	}{
		{method: http.MethodGet, handle: u.get},
		{method: http.MethodPut, handle: u.save},
		{method: http.MethodDelete, handle: u.delete},
	} {
		handle, err := route.handle.WithErrors(kit.ErrorConfig{
			Rules: []kit.ErrorRule{
				{Error: errNotFound, Status: http.StatusNotFound, Message: errNotFound.Error()},
				{Error: errInvalidUser, Status: http.StatusBadRequest, Message: errInvalidUser.Error()},
			},
			InternalMessage: "could not complete user operation",
		})
		if err != nil {
			panic(err)
		}

		router.Method(route.method, "/users/{id}", handle)
	}
}

func (u *Users) get(request kit.Handle) {
	user, err := u.Get(request.Context(), request.Param("id"))
	if err != nil {
		request.Error.Handle(err)
		return
	}

	if err := request.JSON(http.StatusOK, user); err != nil {
		log.Printf("write user response: %v", err)
	}
}

func (u *Users) save(request kit.Handle) {
	var input User
	if err := request.DecodeJSON(&input); err != nil {
		request.Error.BadRequest("invalid JSON body")
		return
	}

	if input.ID != "" && input.ID != request.Param("id") {
		request.Error.BadRequest("body ID must match the requested user")
		return
	}

	input.ID = request.Param("id")

	user, err := u.Save(request.Context(), input)
	if err != nil {
		request.Error.Handle(err)
		return
	}

	if err := request.JSON(http.StatusOK, user); err != nil {
		log.Printf("write user response: %v", err)
	}
}

func (u *Users) delete(request kit.Handle) {
	if err := u.Delete(request.Context(), request.Param("id")); err != nil {
		request.Error.Handle(err)
		return
	}

	request.NoContent()
}
