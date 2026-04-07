package main

func main() {
	router := Router()
	SetupRoutes(router)
	router.Run(":8080")
}
