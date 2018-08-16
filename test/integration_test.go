package test

import (
	"context"
	"errors"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/assert"
)

func Test_Integration(t *testing.T) {

	ctx, cancel := context.WithCancel(context.Background())
	dockerClient, err := client.NewEnvClient()
	defer cancel()

	if err != nil {
		t.Errorf("%v", err)
		return
	}

	netRes, err := dockerClient.NetworkCreate(ctx, "pcp", types.NetworkCreate{
		Driver:     "overlay",
		Attachable: true,
	})
	if err != nil {
		t.Errorf("%v", err)
		return
	}

	tests := []struct {
		name string
		run  func(t *testing.T) (string, bool)
		// Used if need to wait for a result to propogate
		wait func(t *testing.T, timeout time.Duration) bool
		// Set timeout for wait
		timeout time.Duration
	}{
		{
			name: "Startup",
			run: func(t *testing.T) (string, bool) {
				id, err := StartAuxService(ctx, dockerClient, auxServiceSpec)
				if err != nil {
					t.Errorf("%v", err)
					return "", false
				}
				return id, true
			},
			wait: func(t *testing.T, timeout time.Duration) bool {
				var (
					foundAux, startAux, runAux bool = false, false, false
					targetContainer            string
				)

				// Initialize parent context (with timeout)
				timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)

				auxCreated := timoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
					dc, err := client.NewEnvClient()
					if err != nil {
						t.Errorf("%v", err)
						return false
					}
					cntxt := args[0].(context.Context)
					eventChan, errChan := dc.Events(cntxt, types.EventsOptions{})

					for {
						select {
						case <-cntxt.Done():
							return false
						case e := <-errChan:
							log.Println(fmt.Errorf("%v", e))
							return false
						case e := <-eventChan:
							if e.Type != "container" {
								break
							}
							if e.Action != "create" {
								break
							}
							if v, ok := e.Actor.Attributes["name"]; ok {
								if v != "aux-services" {
									break
								}
							} else {
								break
							}
							targetContainer = e.Actor.ID
							return true
						}
						time.Sleep(100 * time.Millisecond)
					}
				})

				auxStarted := timoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
					dc, err := client.NewEnvClient()
					if err != nil {
						t.Errorf("%v", err)
						return false
					}
					cntxt := args[0].(context.Context)
					eventChan, errChan := dc.Events(cntxt, types.EventsOptions{})

					for {
						select {
						case <-cntxt.Done():
							return false
						case e := <-errChan:
							log.Println(fmt.Errorf("%v", e))
							return false
						case e := <-eventChan:
							if e.Type != "container" {
								break
							}
							if e.Action != "start" {
								break
							}
							if v, ok := e.Actor.Attributes["name"]; ok {
								if v != "aux-services" {
									break
								}
							} else {
								break
							}
							targetContainer = e.Actor.ID
							return true
						}
						time.Sleep(100 * time.Millisecond)
					}
				})

				auxRunning := timoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
					dc, err := client.NewEnvClient()
					if err != nil {
						t.Errorf("%v", err)
						return false
					}
					cntxt := args[0].(context.Context)

					resChan := func(ctx context.Context, d *client.Client) <-chan struct{} {
						r := make(chan struct{})
						go func() {
							for {
								insp, err := d.ContainerInspect(ctx, targetContainer)
								if err == nil {
									if insp.State.Status == "running" {
										r <- struct{}{}
									}
								} else if err == errors.New("context canceled") {
									close(r)
									return
								}
								time.Sleep(100 * time.Millisecond)
							}
						}()
						return r
					}(cntxt, dc)

					for {
						select {
						case <-cntxt.Done():
							return false
						case <-resChan:
							return true
						}
					}
				})

				defer cancel()

				// for loop that iterates until context <-Done()
				// once <-Done() then get return from all goroutines
			L:
				for {
					select {
					case <-timeoutCtx.Done():
						<-auxCreated
						<-auxStarted
						<-auxRunning
						log.Printf("Done (main)")
						break L
					case v := <-auxCreated:
						if v {
							log.Printf("Setting foundAux to %v", v)
							foundAux = v
						}
					case v := <-auxStarted:
						if v {
							log.Printf("Setting startAux to %v", v)
							startAux = v
						}
					case v := <-auxRunning:
						if v {
							log.Printf("Setting runAux to %v", v)
							runAux = v
						}
					default:
						break
					}
					if foundAux && startAux && runAux {
						break L
					}
					time.Sleep(100 * time.Millisecond)
				}

				if !foundAux {
					t.Errorf("aux services not found")
				}
				if !startAux {
					t.Errorf("aux services not started")
				}
				if !runAux {
					t.Errorf("aux services not running")
				}

				return foundAux && startAux && runAux
			},
			timeout: 20 * time.Second,
		},
		{
			name: "Kill container",
			run: func(t *testing.T) (string, bool) {
				id, err := StartAuxService(ctx, dockerClient, auxServiceSpec)
				if err != nil {
					t.Errorf("%v", err)
					return "", false
				}
				KillAux(ctx, dockerClient, getAuxID())
				return id, true
			},
			wait: func(t *testing.T, timeout time.Duration) bool {
				var (
					deadAux, startAuxService, startAuxAgain bool = false, false, false
				)

				// Initialize parent context (with timeout)
				timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)

				auxDie := timoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
					dc, err := client.NewEnvClient()
					if err != nil {
						t.Errorf("%v", err)
						return false
					}
					cntxt := args[0].(context.Context)
					eventChan, errChan := dc.Events(cntxt, types.EventsOptions{})

					for {
						select {
						case <-cntxt.Done():
							return false
						case e := <-errChan:
							log.Println(fmt.Errorf("%v", e))
							return false
						case e := <-eventChan:
							if e.Type != "container" {
								break
							}
							if e.Action != "die" {
								break
							}
							if v, ok := e.Actor.Attributes["name"]; ok {
								if v != "aux-services" {
									break
								}
							} else {
								break
							}
							return true
						}
						time.Sleep(100 * time.Millisecond)
					}
				})

				var auxServiceStart <-chan bool
				var auxRestart <-chan bool

				defer cancel()

				// for loop that iterates until context <-Done()
				// once <-Done() then get return from all goroutines
			L:
				for {
					select {
					case <-timeoutCtx.Done():
						<-auxDie
						log.Printf("Done (main)")
						break L
					case v := <-auxDie:
						if v {
							log.Printf("Setting deadAux to %v", v)
							deadAux = v
							auxServiceStart = timoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
								dc, err := client.NewEnvClient()
								if err != nil {
									t.Errorf("%v", err)
									return false
								}
								cntxt := args[0].(context.Context)
								eventChan, errChan := dc.Events(cntxt, types.EventsOptions{})

								for {
									select {
									case <-cntxt.Done():
										return false
									case e := <-errChan:
										log.Println(fmt.Errorf("%v", e))
										return false
									case e := <-eventChan:
										if e.Type != "container" {
											break
										}
										if e.Action != "start" {
											break
										}
										if v, ok := e.Actor.Attributes["com.docker.swarm.service.name"]; ok {
											if v != "AuxiliaryServices" {
												break
											}
										} else {
											break
										}
										return true
									}
									time.Sleep(100 * time.Millisecond)
								}
							})
						}
					case v := <-auxServiceStart:
						if v {
							log.Printf("Setting startAuxService to %v", v)
							startAuxService = v
							auxRestart = timoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
								dc, err := client.NewEnvClient()
								if err != nil {
									t.Errorf("%v", err)
									return false
								}
								cntxt := args[0].(context.Context)
								eventChan, errChan := dc.Events(cntxt, types.EventsOptions{})

								for {
									select {
									case <-cntxt.Done():
										return false
									case e := <-errChan:
										log.Println(fmt.Errorf("%v", e))
										return false
									case e := <-eventChan:
										if e.Type != "container" {
											break
										}
										if e.Action != "start" {
											break
										}
										if v, ok := e.Actor.Attributes["name"]; ok {
											if v != "aux-services" {
												break
											}
										} else {
											break
										}
										return true
									}
									time.Sleep(100 * time.Millisecond)
								}
							})
						}
					case v := <-auxRestart:
						if v {
							log.Printf("Setting startAuxAgain to %v", v)
							startAuxAgain = v
						}
					default:
						break
					}
					if deadAux && startAuxService && startAuxAgain {
						break L
					}
					time.Sleep(100 * time.Millisecond)
				}

				if !deadAux {
					t.Errorf("aux services didn't die")
				}
				if !startAuxService {
					t.Errorf("aux services service didn't restart")
				}
				if !startAuxAgain {
					t.Errorf("aux services didnt' restart")
				}

				return deadAux && startAuxService && startAuxAgain
			},
			timeout: 30 * time.Second,
		},
		{
			name: "Stop",
			run: func(t *testing.T) (string, bool) {
				id, err := StartAuxService(ctx, dockerClient, auxServiceSpec)
				if err != nil {
					t.Errorf("%v", err)
					return "", false
				}
				KillAuxService(ctx, dockerClient, id)
				return id, true
			},
			wait: func(t *testing.T, timeout time.Duration) bool {
				var (
					stopAux, stopAuxService bool = false, false
				)

				// Initialize parent context (with timeout)
				timeoutCtx, cancel := context.WithTimeout(context.Background(), timeout)

				auxStopped := timoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
					dc, err := client.NewEnvClient()
					if err != nil {
						t.Errorf("%v", err)
						return false
					}
					cntxt := args[0].(context.Context)
					eventChan, errChan := dc.Events(cntxt, types.EventsOptions{})

					for {
						select {
						case <-cntxt.Done():
							return false
						case e := <-errChan:
							log.Println(fmt.Errorf("%v", e))
							return false
						case e := <-eventChan:
							if e.Type != "container" {
								break
							}
							if e.Action != "die" {
								break
							}
							if v, ok := e.Actor.Attributes["name"]; ok {
								if v != "aux-services" {
									break
								}
							} else {
								break
							}
							return true
						}
						time.Sleep(100 * time.Millisecond)
					}
				})

				auxServiceStopped := timoutTester(timeoutCtx, []interface{}{timeoutCtx}, func(args ...interface{}) bool {
					dc, err := client.NewEnvClient()
					if err != nil {
						t.Errorf("%v", err)
						return false
					}
					cntxt := args[0].(context.Context)
					eventChan, errChan := dc.Events(cntxt, types.EventsOptions{})

					for {
						select {
						case <-cntxt.Done():
							return false
						case e := <-errChan:
							log.Println(fmt.Errorf("%v", e))
							return false
						case e := <-eventChan:
							log.Printf("event: %+v", e)
							if e.Type != "container" {
								break
							}
							if e.Action != "die" {
								break
							}
							if v, ok := e.Actor.Attributes["com.docker.swarm.service.name"]; ok {
								if v != "AuxiliaryServices" {
									break
								}
							} else {
								break
							}
							return true
						}
						time.Sleep(100 * time.Millisecond)
					}
				})

				defer cancel()

				// for loop that iterates until context <-Done()
				// once <-Done() then get return from all goroutines
			L:
				for {
					select {
					case <-timeoutCtx.Done():
						<-auxServiceStopped
						<-auxStopped
						log.Printf("Done (main)")
						break L
					case v := <-auxServiceStopped:
						if v {
							log.Printf("Setting stopAuxService to %v", v)
							stopAuxService = v
						}
					case v := <-auxStopped:
						if v {
							log.Printf("Setting stopAux to %v", v)
							stopAux = v
						}
					default:
						break
					}
					if stopAuxService && stopAux {
						break L
					}
					time.Sleep(100 * time.Millisecond)
				}

				if !stopAuxService {
					t.Errorf("aux services not stopped")
				}
				if !stopAux {
					t.Errorf("aux not stopped")
				}

				return stopAuxService && stopAux
			},
			timeout: 30 * time.Second,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := make(chan bool)
			go func() {
				res <- tt.wait(t, tt.timeout)
				close(res)
				return
			}()
			id, verify := tt.run(t)
			assert.True(t, verify)
			assert.True(t, <-res)
			KillAuxService(ctx, dockerClient, id)
		})
	}
	err = KillNet(netRes.ID)
	if err != nil {
		t.Errorf("%v", err)
	}
}
