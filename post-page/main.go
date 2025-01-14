package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/go-chi/chi"
	"github.com/unrolled/render"

	postpb "github.com/Jeiwan/micros/post/proto/post"
	wonaming "github.com/wothing/wonaming/consul"
	"google.golang.org/grpc"
)

func postsHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	postClient := ctx.Value("postClient").(postpb.PostServiceClient)
	render := ctx.Value("render").(*render.Render)

	posts, err := postClient.ListPosts(context.Background(), &postpb.ListRequest{})
	if err != nil {
		render.Text(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	render.HTML(w, http.StatusOK, "posts/index", posts.Posts)
}

func newPostHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	render := ctx.Value("render").(*render.Render)

	render.HTML(w, http.StatusOK, "posts/new", nil)
}

func createPostHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	postClient := ctx.Value("postClient").(postpb.PostServiceClient)
	render := ctx.Value("render").(*render.Render)

	post := postpb.Post{
		Title:  r.FormValue("post[title]"),
		Author: r.FormValue("post[author]"),
		Text:   r.FormValue("post[text]"),
	}

	resp, err := postClient.CreatePost(context.Background(), &post)
	if err != nil {
		render.Text(w, http.StatusServiceUnavailable, "Uh-oh")
		return
	}

	if resp.Status {
		http.Redirect(w, r, "/posts", http.StatusMovedPermanently)
	} else {
		render.Text(w, http.StatusServiceUnavailable, "Uh-oh")
	}
}

func postHandler(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	postClient := ctx.Value("postClient").(postpb.PostServiceClient)
	render := ctx.Value("render").(*render.Render)

	postID, err := strconv.Atoi(chi.URLParam(r, "post-id"))
	if err != nil {
		render.Text(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	req := &postpb.GetRequest{PostID: int64(postID)}
	resp, err := postClient.GetPost(context.Background(), req)
	if err != nil {
		render.Text(w, http.StatusServiceUnavailable, err.Error())
		return
	}

	render.HTML(w, http.StatusOK, "posts/get", resp.Post)
}

func main() {
	resolver := wonaming.NewResolver("post")
	balancer := grpc.RoundRobin(resolver)
	conn, err := grpc.Dial("", grpc.WithInsecure(), grpc.WithBalancer(balancer))
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	postClient := postpb.NewPostServiceClient(conn)

	r := chi.NewRouter()
	r.Use(renderCtx)
	r.Use(postClientCtx(postClient))
	r.Get("/posts", postsHandler)
	r.Get("/posts/new", newPostHandler)
	r.Post("/posts", createPostHandler)
	r.Get("/posts/{post-id}", postHandler)

	fmt.Println("Starting the HTTP server")
	http.ListenAndServe(":8080", r)
}

func postClientCtx(client postpb.PostServiceClient) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), "postClient", client)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func renderCtx(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		render := render.New(render.Options{
			Extensions: []string{".tmpl", ".html"},
			Layout:     "layout",
		})
		ctx := context.WithValue(r.Context(), "render", render)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
