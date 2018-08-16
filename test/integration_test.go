package test

import (
	"context"
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
					foundAux, startAux, runAux bool = false, true, true
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
							log.Printf("event: %v", e)
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

				defer cancel()

				// for loop that iterates until context <-Done()
				// once <-Done() then get return from all goroutines
			L:
				for {
					select {
					case <-timeoutCtx.Done():
						<-auxCreated
						log.Printf("Done (main)")
						break L
					case v := <-auxCreated:
						if v {
							log.Printf("Setting foundAux to %v", v)
							foundAux = v
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
