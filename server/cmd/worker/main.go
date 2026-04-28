package main

import "log"

func main() {
	log.Println("Knowvia worker placeholder: inline queue mode runs work inside the API process.")
	log.Println("Next step is to plug Asynq/Redis into this entrypoint for durable background execution.")
}
