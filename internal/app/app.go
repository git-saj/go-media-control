package app

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"go-media-control/internal/media"

	"github.com/maxence-charriere/go-app/v10/pkg/app"
)

type MediaApp struct {
	app.Compo
	MediaList  []media.Media
	Filtered   []media.Media
	SearchTerm string
	ErrorMsg   string
	Page       int
	PageSize   int
	Loading    bool
}

// CustomCSS is now in static/styles.css

func (m *MediaApp) Render() app.UI {
	return app.Div().Class("container my-5").Body(
		app.If(m.ErrorMsg != "",
			func() app.UI { return app.Div().Class("notification is-danger").Text(m.ErrorMsg) },
		),
		app.Div().Class("field mb-5").Body(
			app.Label().Class("label").Text("Search Media"),
			app.Div().Class("control").Body(
				app.Input().
					Class("input").
					Type("text").
					Placeholder("Filter by name...").
					Value(m.SearchTerm).
					OnInput(m.OnSearch),
			),
			app.Button().Class("button is-primary mt-2").Text("Refresh").OnClick(func(ctx app.Context, e app.Event) {
				m.OnRefresh(ctx)
			}),
		),
		app.If(m.Loading,
			func() app.UI {
				return app.Div().Class("has-text-centered py-6").Body(
					app.P().Text("Loading media..."),
					app.Progress().Class("progress is-primary").Value(50).Max(100),
				)
			},
		).Else(
			func() app.UI {
				return app.Div().Body(
					app.If(len(m.Filtered) == 0 && m.ErrorMsg == "",
						func() app.UI { return app.P().Class("has-text-centered").Text("No media found.") },
					).Else(
						func() app.UI {
							// Calculate the items to display for the current page
							start := m.Page * m.PageSize
							end := start + m.PageSize
							if end > len(m.Filtered) {
								end = len(m.Filtered)
							}

							visibleItems := m.Filtered
							if m.PageSize > 0 && len(m.Filtered) > m.PageSize {
								visibleItems = m.Filtered[start:end]
							}

							return app.Div().Body(
								app.Div().Class("media-grid mb-4").Body(
									app.Range(visibleItems).Slice(func(i int) app.UI {
										media := visibleItems[i]
										return app.Div().Class("media-card card").Body(
											app.Div().Class("media-image").Body(
												app.If(media.Logo != "",
													func() app.UI {
														return app.Img().
															Src(media.Logo).
															Alt(media.Name).
															Attr("loading", "lazy")
													},
												).Else(
													func() app.UI {
														return app.Img().
															Src("https://via.placeholder.com/150").
															Alt("No logo").
															Attr("loading", "lazy")
													},
												),
											),
											app.Div().Class("media-name").Text(media.Name),
										).OnClick(func(ctx app.Context, e app.Event) { m.OnCardClick(ctx, media) })
									}),
								),
								// Pagination
								app.If(m.PageSize > 0 && len(m.Filtered) > m.PageSize,
									func() app.UI {
										return app.Nav().Class("pagination is-centered my-5").Body(
											app.A().Class("pagination-previous").
												Text("Previous").
												OnClick(m.PrevPage).
												Attr("disabled", m.Page == 0),
											app.A().Class("pagination-next").
												Text("Next").
												OnClick(m.NextPage).
												Attr("disabled", (m.Page+1)*m.PageSize >= len(m.Filtered)),
											app.Ul().Class("pagination-list").Body(
												app.Li().Text(fmt.Sprintf("Page %d of %d", m.Page+1, (len(m.Filtered)-1)/m.PageSize+1)),
											),
										)
									},
								),
							)
						},
					),
				)
			},
		),
	)
}

func (m *MediaApp) OnSearch(ctx app.Context, e app.Event) {
	m.SearchTerm = ctx.JSSrc().Get("value").String()
	m.Filtered = filterMedia(m.MediaList, m.SearchTerm)
	m.Page = 0 // Reset to first page on search
	ctx.Update()
}

func (m *MediaApp) OnCardClick(ctx app.Context, media media.Media) {
	// Send to API instead of directly calling discord
	mediaJSON, err := json.Marshal(media)
	if err != nil {
		m.ErrorMsg = fmt.Sprintf("Failed to serialize media: %v", err)
		slog.Error("Failed to serialize media", "error", err)
		ctx.Update()
		return
	}

	// Create a new XMLHttpRequest for POST
	req := app.Window().Get("XMLHttpRequest").New()
	req.Call("open", "POST", "/api/send", true)
	req.Set("responseType", "text")
	req.Set("onload", app.FuncOf(func(this app.Value, args []app.Value) interface{} {
		status := req.Get("status").Int()
		if status != http.StatusOK {
			responseText := req.Get("responseText").String()
			ctx.Dispatch(func(ctx app.Context) {
				m.ErrorMsg = fmt.Sprintf("Failed to send to Discord: %s", responseText)
				slog.Error("Discord API error", "status", status, "message", responseText)
				ctx.Update()
			})
		}
		return nil
	}))
	req.Set("onerror", app.FuncOf(func(this app.Value, args []app.Value) interface{} {
		ctx.Dispatch(func(ctx app.Context) {
			m.ErrorMsg = "Failed to send request to server"
			slog.Error("Failed to send request to server")
			ctx.Update()
		})
		return nil
	}))

	req.Call("setRequestHeader", "Content-Type", "application/json")
	req.Call("send", string(mediaJSON))
}

func (m *MediaApp) PrevPage(ctx app.Context, e app.Event) {
	if m.Page > 0 {
		m.Page--
		ctx.Update()
	}
}

func (m *MediaApp) NextPage(ctx app.Context, e app.Event) {
	if (m.Page+1)*m.PageSize < len(m.Filtered) {
		m.Page++
		ctx.Update()
	}
}

func filterMedia(mediaList []media.Media, term string) []media.Media {
	term = strings.ToLower(term)
	if term == "" {
		return mediaList
	}
	var filtered []media.Media
	for _, m := range mediaList {
		if strings.Contains(strings.ToLower(m.Name), term) {
			filtered = append(filtered, m)
		}
	}
	return filtered
}

func (m *MediaApp) OnMount(ctx app.Context) {
	// Check if styles are loaded, and wait for them if not
	styles := app.Window().Get("document").Get("documentElement").Get("dataset").Get("stylesLoaded")
	if styles.String() != "true" {
		slog.Info("Waiting for stylesheets to load...")

		// Simple approach: just wait a moment for styles to load
		go func() {
			time.Sleep(500 * time.Millisecond)
			ctx.Dispatch(func(ctx app.Context) {
				slog.Info("Waiting period complete, loading content")
				m.fetchMedia(ctx, false)
			})
		}()
	} else {
		// Styles already loaded, proceed immediately
		m.fetchMedia(ctx, false)
	}
}

func (m *MediaApp) OnRefresh(ctx app.Context) {
	m.fetchMedia(ctx, true)
}

// Preload an image to make sure it's in the browser cache
func preloadImage(url string) {
	if url == "" {
		return
	}

	// Create a JS image object to preload the image
	img := app.Window().Get("Image").New()
	img.Set("src", url)
}

func (m *MediaApp) fetchMedia(ctx app.Context, forceRefresh bool) {
	// Initialize pagination if not already set
	if m.PageSize == 0 {
		m.PageSize = 100 // Show 100 items per page
		m.Page = 0       // Start at first page
	}

	m.Loading = true
	ctx.Update()

	refreshParam := ""
	if forceRefresh {
		refreshParam = "?refresh=true"
	}

	slog.Info("Fetching media", "forceRefresh", forceRefresh)
	ctx.Async(func() {
		// Create a new XMLHttpRequest for GET
		req := app.Window().Get("XMLHttpRequest").New()
		req.Call("open", "GET", "/api/media"+refreshParam, true)
		req.Set("responseType", "text")

		req.Set("onload", app.FuncOf(func(this app.Value, args []app.Value) interface{} {
			status := req.Get("status").Int()
			if status != http.StatusOK {
				responseText := req.Get("responseText").String()
				ctx.Dispatch(func(ctx app.Context) {
					m.ErrorMsg = fmt.Sprintf("Failed to load media: %s", responseText)
					slog.Error("Failed to load media", "status", status, "response", responseText)
					m.Loading = false
					ctx.Update()
				})
				return nil
			}

			responseText := req.Get("responseText").String()
			var medias []media.Media
			if err := json.Unmarshal([]byte(responseText), &medias); err != nil {
				ctx.Dispatch(func(ctx app.Context) {
					m.ErrorMsg = fmt.Sprintf("Failed to parse media data: %v", err)
					slog.Error("Failed to parse media response", "error", err)
					m.Loading = false
					ctx.Update()
				})
				return nil
			}

			ctx.Dispatch(func(ctx app.Context) {
				m.MediaList = medias
				m.Filtered = filterMedia(medias, m.SearchTerm)
				m.ErrorMsg = ""
				m.Loading = false
				ctx.Update()

				// Preload images for first few items after rendering to improve perceived performance
				go func() {
					// Wait a moment for the UI to render
					time.Sleep(100 * time.Millisecond)

					// Calculate what's visible currently
					start := m.Page * m.PageSize
					end := start + m.PageSize
					if end > len(m.Filtered) {
						end = len(m.Filtered)
					}

					// Preload visible images
					for i := start; i < end && i < len(m.Filtered); i++ {
						if m.Filtered[i].Logo != "" {
							preloadImage(m.Filtered[i].Logo)
						}
					}
				}()
			})
			return nil
		}))

		req.Set("onerror", app.FuncOf(func(this app.Value, args []app.Value) interface{} {
			ctx.Dispatch(func(ctx app.Context) {
				m.ErrorMsg = "Failed to connect to server"
				slog.Error("Failed to connect to server")
				m.Loading = false
				ctx.Update()
			})
			return nil
		}))

		req.Call("send")
	})
}
