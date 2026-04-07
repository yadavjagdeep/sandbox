package main

func main() {
	e := Router()
	SetUpRoutes(e)
	e.Start(":8082")
}