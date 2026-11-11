package users

import (
	"log"
	"net/http"

	"github.com/renniemaharaj/go-api-kit/data"
	"github.com/renniemaharaj/go-api-kit/kit"
)

type handler struct {
	service *service
}

func (h handler) Routes() []data.Route {
	return []data.Route{
		{Method: http.MethodGet, Path: "/{id}", Handler: h.Get},
		{Method: http.MethodPut, Path: "/{id}", Handler: h.Save},
		{Method: http.MethodDelete, Path: "/{id}", Handler: h.Delete},
	}
}

func (h handler) Get(request kit.Handle) {
	user, err := h.service.Get(request.Context(), request.Param("id"))
	if err != nil {
		request.Error.Handle(err)
		return
	}

	if err := request.JSON(http.StatusOK, user); err != nil {
		log.Printf("write user response: %v", err)
	}
}

func (h handler) Save(request kit.Handle) {
	var input User
	if err := request.UnmarshalJSON(&input); err != nil {
		request.Error.BadRequest("invalid JSON body")
		return
	}

	if input.ID != "" && input.ID != request.Param("id") {
		request.Error.BadRequest("body ID must match the requested user")
		return
	}

	input.ID = request.Param("id")

	user, err := h.service.Save(request.Context(), input)
	if err != nil {
		request.Error.Handle(err)
		return
	}

	if err := request.JSON(http.StatusOK, user); err != nil {
		log.Printf("write user response: %v", err)
	}
}

func (h handler) Delete(request kit.Handle) {
	if err := h.service.Delete(request.Context(), request.Param("id")); err != nil {
		request.Error.Handle(err)
		return
	}

	request.NoContent()
}
