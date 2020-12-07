package api2go

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/manyminds/api2go/jsonapi"
	"github.com/manyminds/api2go/routing"
)

const (
	codeInvalidQueryFields  = "API2GO_INVALID_FIELD_QUERY_PARAM"
	defaultContentTypHeader = "application/vnd.api+json"
)

var (
	queryPageRegex   = regexp.MustCompile(`^page\[(\w+)\]$`)
	queryFieldsRegex = regexp.MustCompile(`^fields\[(\w+)\]$`)
)

type information struct {
	prefix   string
	resolver URLResolver
}

func (i information) GetBaseURL() string {
	return i.resolver.GetBaseURL()
}

func (i information) GetPrefix() string {
	return i.prefix
}

type paginationQueryParams struct {
	number, size, offset, limit string
}

func newPaginationQueryParams(r *http.Request) paginationQueryParams {
	var result paginationQueryParams

	queryParams := r.URL.Query()
	result.number = queryParams.Get("page[number]")
	result.size = queryParams.Get("page[size]")
	result.offset = queryParams.Get("page[offset]")
	result.limit = queryParams.Get("page[limit]")

	return result
}

func (p paginationQueryParams) isValid() bool {
	if p.number == "" && p.size == "" && p.offset == "" && p.limit == "" {
		return false
	}

	if p.number != "" && p.size != "" && p.offset == "" && p.limit == "" {
		return true
	}

	if p.number == "" && p.size == "" && p.offset != "" && p.limit != "" {
		return true
	}

	return false
}

func (p paginationQueryParams) getLinks(r *http.Request, count uint, info information) (result jsonapi.Links, err error) {
	result = make(jsonapi.Links)

	params := r.URL.Query()
	prefix := ""
	baseURL := info.GetBaseURL()
	if baseURL != "" {
		prefix = baseURL
	}
	requestURL := fmt.Sprintf("%s%s", prefix, r.URL.Path)

	if p.number != "" {
		// we have number & size params
		var number uint64
		number, err = strconv.ParseUint(p.number, 10, 64)
		if err != nil {
			return
		}

		if p.number != "1" {
			params.Set("page[number]", "1")
			query, _ := url.QueryUnescape(params.Encode())
			result["first"] = jsonapi.Link{Href: fmt.Sprintf("%s?%s", requestURL, query)}

			params.Set("page[number]", strconv.FormatUint(number-1, 10))
			query, _ = url.QueryUnescape(params.Encode())
			result["prev"] = jsonapi.Link{Href: fmt.Sprintf("%s?%s", requestURL, query)}
		}

		// calculate last page number
		var size uint64
		size, err = strconv.ParseUint(p.size, 10, 64)
		if err != nil {
			return
		}
		totalPages := (uint64(count) / size)
		if (uint64(count) % size) != 0 {
			// there is one more page with some len(items) < size
			totalPages++
		}

		if number != totalPages {
			params.Set("page[number]", strconv.FormatUint(number+1, 10))
			query, _ := url.QueryUnescape(params.Encode())
			result["next"] = jsonapi.Link{Href: fmt.Sprintf("%s?%s", requestURL, query)}

			params.Set("page[number]", strconv.FormatUint(totalPages, 10))
			query, _ = url.QueryUnescape(params.Encode())
			result["last"] = jsonapi.Link{Href: fmt.Sprintf("%s?%s", requestURL, query)}
		}
	} else {
		// we have offset & limit params
		var offset, limit uint64
		offset, err = strconv.ParseUint(p.offset, 10, 64)
		if err != nil {
			return
		}
		limit, err = strconv.ParseUint(p.limit, 10, 64)
		if err != nil {
			return
		}

		if p.offset != "0" {
			params.Set("page[offset]", "0")
			query, _ := url.QueryUnescape(params.Encode())
			result["first"] = jsonapi.Link{Href: fmt.Sprintf("%s?%s", requestURL, query)}

			var prevOffset uint64
			if limit > offset {
				prevOffset = 0
			} else {
				prevOffset = offset - limit
			}
			params.Set("page[offset]", strconv.FormatUint(prevOffset, 10))
			query, _ = url.QueryUnescape(params.Encode())
			result["prev"] = jsonapi.Link{Href: fmt.Sprintf("%s?%s", requestURL, query)}
		}

		// check if there are more entries to be loaded
		if (offset + limit) < uint64(count) {
			params.Set("page[offset]", strconv.FormatUint(offset+limit, 10))
			query, _ := url.QueryUnescape(params.Encode())
			result["next"] = jsonapi.Link{Href: fmt.Sprintf("%s?%s", requestURL, query)}

			params.Set("page[offset]", strconv.FormatUint(uint64(count)-limit, 10))
			query, _ = url.QueryUnescape(params.Encode())
			result["last"] = jsonapi.Link{Href: fmt.Sprintf("%s?%s", requestURL, query)}
		}
	}

	return
}

type notAllowedHandler struct {
	API *API
}

func (n notAllowedHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := NewHTTPError(nil, "Method Not Allowed", http.StatusMethodNotAllowed)
	w.WriteHeader(http.StatusMethodNotAllowed)

	contentType := defaultContentTypHeader
	if n.API != nil {
		contentType = n.API.ContentType
	}

	handleError(err, w, r, contentType)
}

type resource struct {
	resourceType reflect.Type
	source       interface{}
	name         string
	api          *API
}

// middlewareChain executes the middleeware chain setup
func (api *API) middlewareChain(c APIContexter, w http.ResponseWriter, r *http.Request) {
	for _, middleware := range api.middlewares {
		middleware(c, w, r)
	}
}

// allocateContext creates a context for the api.contextPool, saving allocations
func (api *API) allocateDefaultContext() APIContexter {
	return &APIContext{}
}

func (api *API) addResource(prototype jsonapi.MarshalIdentifier, source interface{}) *resource {
	resourceType := reflect.TypeOf(prototype)
	if resourceType.Kind() != reflect.Struct && resourceType.Kind() != reflect.Ptr {
		panic("pass an empty resource struct or a struct pointer to AddResource!")
	}

	var ptrPrototype interface{}
	var name string

	if resourceType.Kind() == reflect.Struct {
		ptrPrototype = reflect.New(resourceType).Interface()
		name = resourceType.Name()
	} else {
		ptrPrototype = reflect.ValueOf(prototype).Interface()
		name = resourceType.Elem().Name()
	}

	// check if EntityNamer interface is implemented and use that as name
	entityName, ok := prototype.(jsonapi.EntityNamer)
	if ok {
		name = entityName.GetName()
	} else {
		name = jsonapi.Jsonify(jsonapi.Pluralize(name))
	}

	res := resource{
		resourceType: resourceType,
		name:         name,
		source:       source,
		api:          api,
	}

	requestInfo := func(r *http.Request, api *API) *information {
		var info *information
		if resolver, ok := api.info.resolver.(RequestAwareURLResolver); ok {
			resolver.SetRequest(*r)
			info = &information{prefix: api.info.prefix, resolver: resolver}
		} else {
			info = &api.info
		}

		return info
	}

	prefix := strings.Trim(api.info.prefix, "/")
	baseURL := "/" + name
	if prefix != "" {
		baseURL = "/" + prefix + baseURL
	}

	api.router.Handle("OPTIONS", baseURL, func(w http.ResponseWriter, r *http.Request, _ map[string]string, context map[string]interface{}) {
		c := api.contextPool.Get().(APIContexter)
		c.Reset()

		for key, val := range context {
			c.Set(key, val)
		}

		api.middlewareChain(c, w, r)
		w.Header().Set("Allow", strings.Join(getAllowedMethods(source, true), ","))
		w.WriteHeader(http.StatusNoContent)
		api.contextPool.Put(c)
	})

	api.router.Handle("GET", baseURL, func(w http.ResponseWriter, r *http.Request, _ map[string]string, context map[string]interface{}) {
		info := requestInfo(r, api)
		c := api.contextPool.Get().(APIContexter)
		c.Reset()

		for key, val := range context {
			c.Set(key, val)
		}

		api.middlewareChain(c, w, r)

		err := res.handleIndex(c, w, r, *info)
		api.contextPool.Put(c)
		if err != nil {
			handleError(err, w, r, api.ContentType)
		}
	})

	if _, ok := source.(ResourceGetter); ok {
		api.router.Handle("OPTIONS", baseURL+"/:id", func(w http.ResponseWriter, r *http.Request, _ map[string]string, context map[string]interface{}) {
			c := api.contextPool.Get().(APIContexter)
			c.Reset()

			for key, val := range context {
				c.Set(key, val)
			}

			api.middlewareChain(c, w, r)
			w.Header().Set("Allow", strings.Join(getAllowedMethods(source, false), ","))
			w.WriteHeader(http.StatusNoContent)
			api.contextPool.Put(c)
		})

		api.router.Handle("GET", baseURL+"/:id", func(w http.ResponseWriter, r *http.Request, params map[string]string, context map[string]interface{}) {
			info := requestInfo(r, api)
			c := api.contextPool.Get().(APIContexter)
			c.Reset()

			for key, val := range context {
				c.Set(key, val)
			}

			api.middlewareChain(c, w, r)
			err := res.handleRead(c, w, r, params, *info)
			api.contextPool.Put(c)
			if err != nil {
				handleError(err, w, r, api.ContentType)
			}
		})
	}

	// generate all routes for linked relations if there are relations
	casted, ok := prototype.(jsonapi.MarshalReferences)
	if ok {
		relations := casted.GetReferences()
		for _, relation := range relations {
			api.router.Handle("GET", baseURL+"/:id/relationships/"+relation.Name, func(relation jsonapi.Reference) routing.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request, params map[string]string, context map[string]interface{}) {
					info := requestInfo(r, api)
					c := api.contextPool.Get().(APIContexter)
					c.Reset()

					for key, val := range context {
						c.Set(key, val)
					}

					api.middlewareChain(c, w, r)
					err := res.handleReadRelation(c, w, r, params, *info, relation)
					api.contextPool.Put(c)
					if err != nil {
						handleError(err, w, r, api.ContentType)
					}
				}
			}(relation))

			api.router.Handle("GET", baseURL+"/:id/"+relation.Name, func(relation jsonapi.Reference) routing.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request, params map[string]string, context map[string]interface{}) {
					info := requestInfo(r, api)
					c := api.contextPool.Get().(APIContexter)
					c.Reset()

					for key, val := range context {
						c.Set(key, val)
					}

					api.middlewareChain(c, w, r)
					err := res.handleLinked(c, api, w, r, params, relation, *info)
					api.contextPool.Put(c)
					if err != nil {
						handleError(err, w, r, api.ContentType)
					}
				}
			}(relation))

			api.router.Handle("PATCH", baseURL+"/:id/relationships/"+relation.Name, func(relation jsonapi.Reference) routing.HandlerFunc {
				return func(w http.ResponseWriter, r *http.Request, params map[string]string, context map[string]interface{}) {
					c := api.contextPool.Get().(APIContexter)
					c.Reset()

					for key, val := range context {
						c.Set(key, val)
					}

					api.middlewareChain(c, w, r)
					err := res.handleReplaceRelation(c, w, r, params, relation)
					api.contextPool.Put(c)
					if err != nil {
						handleError(err, w, r, api.ContentType)
					}
				}
			}(relation))

			if _, ok := ptrPrototype.(jsonapi.EditToManyRelations); ok && relation.Name == jsonapi.Pluralize(relation.Name) {
				// generate additional routes to manipulate to-many relationships
				api.router.Handle("POST", baseURL+"/:id/relationships/"+relation.Name, func(relation jsonapi.Reference) routing.HandlerFunc {
					return func(w http.ResponseWriter, r *http.Request, params map[string]string, context map[string]interface{}) {
						c := api.contextPool.Get().(APIContexter)
						c.Reset()

						for key, val := range context {
							c.Set(key, val)
						}

						api.middlewareChain(c, w, r)
						err := res.handleAddToManyRelation(c, w, r, params, relation)
						api.contextPool.Put(c)
						if err != nil {
							handleError(err, w, r, api.ContentType)
						}
					}
				}(relation))

				api.router.Handle("DELETE", baseURL+"/:id/relationships/"+relation.Name, func(relation jsonapi.Reference) routing.HandlerFunc {
					return func(w http.ResponseWriter, r *http.Request, params map[string]string, context map[string]interface{}) {
						c := api.contextPool.Get().(APIContexter)
						c.Reset()

						for key, val := range context {
							c.Set(key, val)
						}

						api.middlewareChain(c, w, r)
						err := res.handleDeleteToManyRelation(c, w, r, params, relation)
						api.contextPool.Put(c)
						if err != nil {
							handleError(err, w, r, api.ContentType)
						}
					}
				}(relation))
			}
		}
	}

	if _, ok := source.(ResourceCreator); ok {
		api.router.Handle("POST", baseURL, func(w http.ResponseWriter, r *http.Request, params map[string]string, context map[string]interface{}) {
			info := requestInfo(r, api)
			c := api.contextPool.Get().(APIContexter)
			c.Reset()

			for key, val := range context {
				c.Set(key, val)
			}

			api.middlewareChain(c, w, r)
			err := res.handleCreate(c, w, r, info.prefix, *info)
			api.contextPool.Put(c)
			if err != nil {
				handleError(err, w, r, api.ContentType)
			}
		})
	}

	if _, ok := source.(ResourceDeleter); ok {
		api.router.Handle("DELETE", baseURL+"/:id", func(w http.ResponseWriter, r *http.Request, params map[string]string, context map[string]interface{}) {
			c := api.contextPool.Get().(APIContexter)
			c.Reset()

			for key, val := range context {
				c.Set(key, val)
			}

			api.middlewareChain(c, w, r)
			err := res.handleDelete(c, w, r, params)
			api.contextPool.Put(c)
			if err != nil {
				handleError(err, w, r, api.ContentType)
			}
		})
	}

	if _, ok := source.(ResourceUpdater); ok {
		api.router.Handle("PATCH", baseURL+"/:id", func(w http.ResponseWriter, r *http.Request, params map[string]string, context map[string]interface{}) {
			info := requestInfo(r, api)
			c := api.contextPool.Get().(APIContexter)
			c.Reset()

			for key, val := range context {
				c.Set(key, val)
			}

			api.middlewareChain(c, w, r)
			err := res.handleUpdate(c, w, r, params, *info)
			api.contextPool.Put(c)
			if err != nil {
				handleError(err, w, r, api.ContentType)
			}
		})
	}

	api.resources = append(api.resources, res)

	return &res
}

func getAllowedMethods(source interface{}, collection bool) []string {
	result := []string{http.MethodOptions}

	if _, ok := source.(ResourceGetter); ok {
		result = append(result, http.MethodGet)
	}

	if _, ok := source.(ResourceUpdater); ok {
		result = append(result, http.MethodPatch)
	}

	if _, ok := source.(ResourceDeleter); ok && !collection {
		result = append(result, http.MethodDelete)
	}

	if _, ok := source.(ResourceCreator); ok && collection {
		result = append(result, http.MethodPost)
	}

	return result
}

func buildRequest(c APIContexter, r *http.Request) Request {
	req := Request{PlainRequest: r}
	params := make(map[string][]string)
	pagination := make(map[string]string)
	for key, values := range r.URL.Query() {
		params[key] = strings.Split(values[0], ",")
		pageMatches := queryPageRegex.FindStringSubmatch(key)
		if len(pageMatches) > 1 {
			pagination[pageMatches[1]] = values[0]
		}
	}
	req.Pagination = pagination
	req.QueryParams = params
	req.Header = r.Header
	req.Context = c
	return req
}

func (res *resource) marshalResponse(resp interface{}, w http.ResponseWriter, status int, r *http.Request) error {
	filtered, err := filterSparseFields(resp, r)
	if err != nil {
		return err
	}
	result, err := json.Marshal(filtered)
	if err != nil {
		return err
	}
	writeResult(w, result, status, res.api.ContentType)
	return nil
}

func (res *resource) handleIndex(c APIContexter, w http.ResponseWriter, r *http.Request, info information) error {
	if source, ok := res.source.(PaginatedFindAll); ok {
		pagination := newPaginationQueryParams(r)

		if pagination.isValid() {
			count, response, err := source.PaginatedFindAll(buildRequest(c, r))
			if err != nil {
				return err
			}

			paginationLinks, err := pagination.getLinks(r, count, info)
			if err != nil {
				return err
			}

			return res.respondWithPagination(response, info, http.StatusOK, paginationLinks, w, r)
		}
	}

	source, ok := res.source.(FindAll)
	if !ok {
		return NewHTTPError(nil, "Resource does not implement the FindAll interface", http.StatusNotFound)
	}

	response, err := source.FindAll(buildRequest(c, r))
	if err != nil {
		return err
	}

	return res.respondWith(response, info, http.StatusOK, w, r)
}

func (res *resource) handleRead(c APIContexter, w http.ResponseWriter, r *http.Request, params map[string]string, info information) error {
	source, ok := res.source.(ResourceGetter)

	if !ok {
		return fmt.Errorf("Resource %s does not implement the ResourceGetter interface", res.name)
	}

	id := params["id"]

	response, err := source.FindOne(id, buildRequest(c, r))

	if err != nil {
		return err
	}

	return res.respondWith(response, info, http.StatusOK, w, r)
}

func (res *resource) handleReadRelation(c APIContexter, w http.ResponseWriter, r *http.Request, params map[string]string, info information, relation jsonapi.Reference) error {
	source, ok := res.source.(ResourceGetter)

	if !ok {
		return fmt.Errorf("Resource %s does not implement the ResourceGetter interface", res.name)
	}

	id := params["id"]

	obj, err := source.FindOne(id, buildRequest(c, r))
	if err != nil {
		return err
	}

	document, err := jsonapi.MarshalToStruct(obj.Result(), info)
	if err != nil {
		return err
	}

	rel, ok := document.Data.DataObject.Relationships[relation.Name]
	if !ok {
		return NewHTTPError(nil, fmt.Sprintf("There is no relation with the name %s", relation.Name), http.StatusNotFound)
	}

	meta := obj.Metadata()
	if len(meta) > 0 {
		rel.Meta = meta
	}

	return res.marshalResponse(rel, w, http.StatusOK, r)
}

// try to find the referenced resource and call the findAll Method with referencing resource id as param
func (res *resource) handleLinked(c APIContexter, api *API, w http.ResponseWriter, r *http.Request, params map[string]string, linked jsonapi.Reference, info information) error {
	id := params["id"]
	for _, resource := range api.resources {
		if resource.name == linked.Type {
			request := buildRequest(c, r)
			request.QueryParams[res.name+"ID"] = []string{id}
			request.QueryParams[res.name+"Name"] = []string{linked.Name}

			if source, ok := resource.source.(PaginatedFindAll); ok {
				// check for pagination, otherwise normal FindAll
				pagination := newPaginationQueryParams(r)
				if pagination.isValid() {
					var count uint
					count, response, err := source.PaginatedFindAll(request)
					if err != nil {
						return err
					}

					paginationLinks, err := pagination.getLinks(r, count, info)
					if err != nil {
						return err
					}

					return res.respondWithPagination(response, info, http.StatusOK, paginationLinks, w, r)
				}
			}

			source, ok := resource.source.(FindAll)
			if !ok {
				return NewHTTPError(nil, "Resource does not implement the FindAll interface", http.StatusNotFound)
			}

			obj, err := source.FindAll(request)
			if err != nil {
				return err
			}
			return res.respondWith(obj, info, http.StatusOK, w, r)
		}
	}

	return NewHTTPError(
		errors.New("Not Found"),
		"No resource handler is registered to handle the linked resource "+linked.Name,
		http.StatusNotFound,
	)
}

func (res *resource) handleCreate(c APIContexter, w http.ResponseWriter, r *http.Request, prefix string, info information) error {
	source, ok := res.source.(ResourceCreator)

	if !ok {
		return fmt.Errorf("Resource %s does not implement the ResourceCreator interface", res.name)
	}

	ctx, err := unmarshalRequest(r)
	if err != nil {
		return err
	}

	// Ok this is weird again, but reflect.New produces a pointer, so we need the pure type without pointer,
	// otherwise we would have a pointer pointer type that we don't want.
	resourceType := res.resourceType
	if resourceType.Kind() == reflect.Ptr {
		resourceType = resourceType.Elem()
	}
	newObj := reflect.New(resourceType).Interface()

	// Call InitializeObject if available to allow implementers change the object
	// before calling Unmarshal.
	if initSource, ok := source.(ObjectInitializer); ok {
		initSource.InitializeObject(newObj)
	}

	err = jsonapi.Unmarshal(ctx, newObj)
	if err != nil {
		return NewHTTPError(nil, err.Error(), http.StatusNotAcceptable)
	}

	var response Responder

	if res.resourceType.Kind() == reflect.Struct {
		// we have to dereference the pointer if user wants to use non pointer values
		response, err = source.Create(reflect.ValueOf(newObj).Elem().Interface(), buildRequest(c, r))
	} else {
		response, err = source.Create(newObj, buildRequest(c, r))
	}
	if err != nil {
		return err
	}

	result, ok := response.Result().(jsonapi.MarshalIdentifier)

	if !ok {
		return fmt.Errorf("Expected one newly created object by resource %s", res.name)
	}

	if len(prefix) > 0 {
		w.Header().Set("Location", "/"+prefix+"/"+res.name+"/"+result.GetID())
	} else {
		w.Header().Set("Location", "/"+res.name+"/"+result.GetID())
	}

	// handle 200 status codes
	switch response.StatusCode() {
	case http.StatusCreated:
		return res.respondWith(response, info, http.StatusCreated, w, r)
	case http.StatusNoContent:
		w.WriteHeader(response.StatusCode())
		return nil
	case http.StatusAccepted:
		w.WriteHeader(response.StatusCode())
		return nil
	default:
		return fmt.Errorf("invalid status code %d from resource %s for method Create", response.StatusCode(), res.name)
	}
}

func (res *resource) handleUpdate(c APIContexter, w http.ResponseWriter, r *http.Request, params map[string]string, info information) error {
	source, ok := res.source.(ResourceUpdater)

	if !ok {
		return fmt.Errorf("Resource %s does not implement the ResourceUpdater interface", res.name)
	}

	id := params["id"]
	obj, err := source.FindOne(id, buildRequest(c, r))
	if err != nil {
		return err
	}

	ctx, err := unmarshalRequest(r)
	if err != nil {
		return err
	}

	// we have to make the Result to a pointer to unmarshal into it
	updatingObj := reflect.ValueOf(obj.Result())
	if updatingObj.Kind() == reflect.Struct {
		updatingObjPtr := reflect.New(reflect.TypeOf(obj.Result()))
		updatingObjPtr.Elem().Set(updatingObj)
		err = jsonapi.Unmarshal(ctx, updatingObjPtr.Interface())
		updatingObj = updatingObjPtr.Elem()
	} else {
		err = jsonapi.Unmarshal(ctx, updatingObj.Interface())
	}
	if err != nil {
		return NewHTTPError(nil, err.Error(), http.StatusNotAcceptable)
	}

	identifiable, ok := updatingObj.Interface().(jsonapi.MarshalIdentifier)
	if !ok || identifiable.GetID() != id {
		conflictError := errors.New("id in the resource does not match servers endpoint")
		return NewHTTPError(conflictError, conflictError.Error(), http.StatusConflict)
	}

	response, err := source.Update(updatingObj.Interface(), buildRequest(c, r))

	if err != nil {
		return err
	}

	switch response.StatusCode() {
	case http.StatusOK:
		updated := response.Result()
		if updated == nil {
			internalResponse, err := source.FindOne(id, buildRequest(c, r))
			if err != nil {
				return err
			}
			updated = internalResponse.Result()
			if updated == nil {
				return fmt.Errorf("Expected FindOne to return one object of resource %s", res.name)
			}

			response = internalResponse
		}

		return res.respondWith(response, info, http.StatusOK, w, r)
	case http.StatusAccepted:
		w.WriteHeader(http.StatusAccepted)
		return nil
	case http.StatusNoContent:
		w.WriteHeader(http.StatusNoContent)
		return nil
	default:
		return fmt.Errorf("invalid status code %d from resource %s for method Update", response.StatusCode(), res.name)
	}
}

func (res *resource) handleReplaceRelation(c APIContexter, w http.ResponseWriter, r *http.Request, params map[string]string, relation jsonapi.Reference) error {
	source, ok := res.source.(ResourceUpdater)

	if !ok {
		return fmt.Errorf("Resource %s does not implement the ResourceUpdater interface", res.name)
	}

	var (
		err     error
		editObj interface{}
	)

	id := params["id"]

	response, err := source.FindOne(id, buildRequest(c, r))
	if err != nil {
		return err
	}

	body, err := unmarshalRequest(r)
	if err != nil {
		return err
	}

	inc := map[string]interface{}{}
	err = json.Unmarshal(body, &inc)
	if err != nil {
		return err
	}
	data, ok := inc["data"]
	if !ok {
		return errors.New("Invalid object. Need a \"data\" object")
	}

	resType := reflect.TypeOf(response.Result()).Kind()
	if resType == reflect.Struct {
		editObj = getPointerToStruct(response.Result())
	} else {
		editObj = response.Result()
	}

	err = processRelationshipsData(data, relation.Name, editObj)
	if err != nil {
		return err
	}

	if resType == reflect.Struct {
		_, err = source.Update(reflect.ValueOf(editObj).Elem().Interface(), buildRequest(c, r))
	} else {
		_, err = source.Update(editObj, buildRequest(c, r))
	}

	w.WriteHeader(http.StatusNoContent)
	return err
}

func (res *resource) handleAddToManyRelation(c APIContexter, w http.ResponseWriter, r *http.Request, params map[string]string, relation jsonapi.Reference) error {
	source, ok := res.source.(ResourceUpdater)

	if !ok {
		return fmt.Errorf("Resource %s does not implement the ResourceUpdater interface", res.name)
	}

	var (
		err     error
		editObj interface{}
	)

	id := params["id"]

	response, err := source.FindOne(id, buildRequest(c, r))
	if err != nil {
		return err
	}

	body, err := unmarshalRequest(r)
	if err != nil {
		return err
	}
	inc := map[string]interface{}{}
	err = json.Unmarshal(body, &inc)
	if err != nil {
		return err
	}

	data, ok := inc["data"]
	if !ok {
		return errors.New("Invalid object. Need a \"data\" object")
	}

	newRels, ok := data.([]interface{})
	if !ok {
		return fmt.Errorf("Data must be an array with \"id\" and \"type\" field to add new to-many relationships")
	}

	newIDs := []string{}

	for _, newRel := range newRels {
		casted, ok := newRel.(map[string]interface{})
		if !ok {
			return errors.New("entry in data object invalid")
		}
		newID, ok := casted["id"].(string)
		if !ok {
			return errors.New("no id field found inside data object")
		}

		newIDs = append(newIDs, newID)
	}

	resType := reflect.TypeOf(response.Result()).Kind()
	if resType == reflect.Struct {
		editObj = getPointerToStruct(response.Result())
	} else {
		editObj = response.Result()
	}

	targetObj, ok := editObj.(jsonapi.EditToManyRelations)
	if !ok {
		return errors.New("target struct must implement jsonapi.EditToManyRelations")
	}
	targetObj.AddToManyIDs(relation.Name, newIDs)

	if resType == reflect.Struct {
		_, err = source.Update(reflect.ValueOf(targetObj).Elem().Interface(), buildRequest(c, r))
	} else {
		_, err = source.Update(targetObj, buildRequest(c, r))
	}

	w.WriteHeader(http.StatusNoContent)

	return err
}

func (res *resource) handleDeleteToManyRelation(c APIContexter, w http.ResponseWriter, r *http.Request, params map[string]string, relation jsonapi.Reference) error {
	source, ok := res.source.(ResourceUpdater)

	if !ok {
		return fmt.Errorf("Resource %s does not implement the ResourceUpdater interface", res.name)
	}

	var (
		err     error
		editObj interface{}
	)

	id := params["id"]

	response, err := source.FindOne(id, buildRequest(c, r))
	if err != nil {
		return err
	}

	body, err := unmarshalRequest(r)
	if err != nil {
		return err
	}

	inc := map[string]interface{}{}
	err = json.Unmarshal(body, &inc)
	if err != nil {
		return err
	}

	data, ok := inc["data"]
	if !ok {
		return errors.New("Invalid object. Need a \"data\" object")
	}

	newRels, ok := data.([]interface{})
	if !ok {
		return fmt.Errorf("Data must be an array with \"id\" and \"type\" field to add new to-many relationships")
	}

	obsoleteIDs := []string{}

	for _, newRel := range newRels {
		casted, ok := newRel.(map[string]interface{})
		if !ok {
			return errors.New("entry in data object invalid")
		}
		obsoleteID, ok := casted["id"].(string)
		if !ok {
			return errors.New("no id field found inside data object")
		}

		obsoleteIDs = append(obsoleteIDs, obsoleteID)
	}

	resType := reflect.TypeOf(response.Result()).Kind()
	if resType == reflect.Struct {
		editObj = getPointerToStruct(response.Result())
	} else {
		editObj = response.Result()
	}

	targetObj, ok := editObj.(jsonapi.EditToManyRelations)
	if !ok {
		return errors.New("target struct must implement jsonapi.EditToManyRelations")
	}
	targetObj.DeleteToManyIDs(relation.Name, obsoleteIDs)

	if resType == reflect.Struct {
		_, err = source.Update(reflect.ValueOf(targetObj).Elem().Interface(), buildRequest(c, r))
	} else {
		_, err = source.Update(targetObj, buildRequest(c, r))
	}

	w.WriteHeader(http.StatusNoContent)

	return err
}

// returns a pointer to an interface{} struct
func getPointerToStruct(oldObj interface{}) interface{} {
	resType := reflect.TypeOf(oldObj)
	ptr := reflect.New(resType)
	ptr.Elem().Set(reflect.ValueOf(oldObj))
	return ptr.Interface()
}

func (res *resource) handleDelete(c APIContexter, w http.ResponseWriter, r *http.Request, params map[string]string) error {
	source, ok := res.source.(ResourceDeleter)

	if !ok {
		return fmt.Errorf("Resource %s does not implement the ResourceDeleter interface", res.name)
	}

	id := params["id"]
	response, err := source.Delete(id, buildRequest(c, r))
	if err != nil {
		return err
	}

	switch response.StatusCode() {
	case http.StatusOK:
		data := map[string]interface{}{
			"meta": response.Metadata(),
		}

		return res.marshalResponse(data, w, http.StatusOK, r)
	case http.StatusAccepted:
		w.WriteHeader(http.StatusAccepted)
		return nil
	case http.StatusNoContent:
		w.WriteHeader(http.StatusNoContent)
		return nil
	default:
		return fmt.Errorf("invalid status code %d from resource %s for method Delete", response.StatusCode(), res.name)
	}
}

func writeResult(w http.ResponseWriter, data []byte, status int, contentType string) {
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	w.Write(data)
}

func (res *resource) respondWith(obj Responder, info information, status int, w http.ResponseWriter, r *http.Request) error {
	data, err := jsonapi.MarshalToStruct(obj.Result(), info)
	if err != nil {
		return err
	}

	meta := obj.Metadata()
	if len(meta) > 0 {
		data.Meta = meta
	}

	if objWithLinks, ok := obj.(LinksResponder); ok {
		baseURL := strings.Trim(info.GetBaseURL(), "/")
		requestURL := fmt.Sprintf("%s%s", baseURL, r.URL.Path)
		links := objWithLinks.Links(r, requestURL)
		if len(links) > 0 {
			data.Links = links
		}
	}

	return res.marshalResponse(data, w, status, r)
}

func (res *resource) respondWithPagination(obj Responder, info information, status int, links jsonapi.Links, w http.ResponseWriter, r *http.Request) error {
	data, err := jsonapi.MarshalToStruct(obj.Result(), info)
	if err != nil {
		return err
	}

	data.Links = links
	meta := obj.Metadata()
	if len(meta) > 0 {
		data.Meta = meta
	}

	return res.marshalResponse(data, w, status, r)
}

func unmarshalRequest(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	data, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func filterSparseFields(resp interface{}, r *http.Request) (interface{}, error) {
	query := r.URL.Query()
	queryParams := parseQueryFields(&query)
	if len(queryParams) < 1 {
		return resp, nil
	}

	if document, ok := resp.(*jsonapi.Document); ok {
		wrongFields := map[string][]string{}

		// single entry in data
		data := document.Data.DataObject
		if data != nil {
			errors := replaceAttributes(&queryParams, data)
			for t, v := range errors {
				wrongFields[t] = v
			}
		}

		// data can be a slice too
		datas := document.Data.DataArray
		for index, data := range datas {
			errors := replaceAttributes(&queryParams, &data)
			for t, v := range errors {
				wrongFields[t] = v
			}
			datas[index] = data
		}

		// included slice
		for index, include := range document.Included {
			errors := replaceAttributes(&queryParams, &include)
			for t, v := range errors {
				wrongFields[t] = v
			}
			document.Included[index] = include
		}

		if len(wrongFields) > 0 {
			httpError := NewHTTPError(nil, "Some requested fields were invalid", http.StatusBadRequest)
			for k, v := range wrongFields {
				for _, field := range v {
					httpError.Errors = append(httpError.Errors, Error{
						Status: "Bad Request",
						Code:   codeInvalidQueryFields,
						Title:  fmt.Sprintf(`Field "%s" does not exist for type "%s"`, field, k),
						Detail: "Please make sure you do only request existing fields",
						Source: &ErrorSource{
							Parameter: fmt.Sprintf("fields[%s]", k),
						},
					})
				}
			}
			return nil, httpError
		}
	}
	return resp, nil
}

func parseQueryFields(query *url.Values) (result map[string][]string) {
	result = map[string][]string{}
	for name, param := range *query {
		matches := queryFieldsRegex.FindStringSubmatch(name)
		if len(matches) > 1 {
			match := matches[1]
			result[match] = strings.Split(param[0], ",")
		}
	}

	return
}

func filterAttributes(attributes map[string]interface{}, fields []string) (filteredAttributes map[string]interface{}, wrongFields []string) {
	wrongFields = []string{}
	filteredAttributes = map[string]interface{}{}

	for _, field := range fields {
		if attribute, ok := attributes[field]; ok {
			filteredAttributes[field] = attribute
		} else {
			wrongFields = append(wrongFields, field)
		}
	}

	return
}

func replaceAttributes(query *map[string][]string, entry *jsonapi.Data) map[string][]string {
	fieldType := entry.Type
	attributes := map[string]interface{}{}
	_ = json.Unmarshal(entry.Attributes, &attributes)
	fields := (*query)[fieldType]
	if len(fields) > 0 {
		var wrongFields []string
		attributes, wrongFields = filterAttributes(attributes, fields)
		if len(wrongFields) > 0 {
			return map[string][]string{
				fieldType: wrongFields,
			}
		}
		bytes, _ := json.Marshal(attributes)
		entry.Attributes = bytes
	}

	return nil
}

func handleError(err error, w http.ResponseWriter, r *http.Request, contentType string) {
	log.Println(err)
	if e, ok := err.(HTTPError); ok {
		writeResult(w, []byte(marshalHTTPError(e)), e.status, contentType)
		return

	}

	e := NewHTTPError(err, err.Error(), http.StatusInternalServerError)
	writeResult(w, []byte(marshalHTTPError(e)), http.StatusInternalServerError, contentType)
}

// TODO: this can also be replaced with a struct into that we directly json.Unmarshal
func processRelationshipsData(data interface{}, linkName string, target interface{}) error {
	hasOne, ok := data.(map[string]interface{})
	if ok {
		hasOneID, ok := hasOne["id"].(string)
		if !ok {
			return fmt.Errorf("data object must have a field id for %s", linkName)
		}

		target, ok := target.(jsonapi.UnmarshalToOneRelations)
		if !ok {
			return errors.New("target struct must implement interface UnmarshalToOneRelations")
		}

		err := target.SetToOneReferenceID(linkName, hasOneID)
		if err != nil {
			return err
		}
	} else if data == nil {
		// this means that a to-one relationship must be deleted
		target, ok := target.(jsonapi.UnmarshalToOneRelations)
		if !ok {
			return errors.New("target struct must implement interface UnmarshalToOneRelations")
		}

		err := target.SetToOneReferenceID(linkName, "")
		if err != nil {
			return err
		}
	} else {
		hasMany, ok := data.([]interface{})
		if !ok {
			return fmt.Errorf("invalid data object or array, must be an object with \"id\" and \"type\" field for %s", linkName)
		}

		target, ok := target.(jsonapi.UnmarshalToManyRelations)
		if !ok {
			return errors.New("target struct must implement interface UnmarshalToManyRelations")
		}

		hasManyIDs := []string{}

		for _, entry := range hasMany {
			data, ok := entry.(map[string]interface{})
			if !ok {
				return fmt.Errorf("entry in data array must be an object for %s", linkName)
			}
			dataID, ok := data["id"].(string)
			if !ok {
				return fmt.Errorf("all data objects must have a field id for %s", linkName)
			}

			hasManyIDs = append(hasManyIDs, dataID)
		}

		err := target.SetToManyReferenceIDs(linkName, hasManyIDs)
		if err != nil {
			return err
		}
	}

	return nil
}
