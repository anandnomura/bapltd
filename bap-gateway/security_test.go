package main

import (
 "net/http"
 "net/http/httptest"
 "testing"
)

func TestConsumeModeNeverFallsBackWhenControlPlaneUnavailable(t *testing.T){
 for _,status:=range []int{http.StatusOK,http.StatusForbidden,http.StatusInternalServerError}{
  server:=httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.WriteHeader(status);_,_=w.Write([]byte(`{"invalid":"response"}`))}))
  cfg:=GatewayConfig{ControlPlane:server.URL,UseConsume:true,SecretKey:"test"}
  ok,_,err:=validateGrant(cfg,"anything","/api/v1/financial-records")
  server.Close()
  if ok || err==nil{t.Fatalf("status %d allowed fallback",status)}
 }
 server:=httptest.NewServer(http.NotFoundHandler());server.Close()
 ok,_,err:=validateGrant(GatewayConfig{ControlPlane:server.URL,UseConsume:true},"anything","/api/v1/financial-records")
 if ok || err==nil{t.Fatal("connection failure allowed fallback")}
}
