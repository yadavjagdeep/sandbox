package main

func main() {
	router := Router()
	SetUpRoutes(router)
	router.Start(":8081")
}