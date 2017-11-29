package antch

// PipelineHandler is an interface for a handler in pipeline.
type PipelineHandler interface {
	ServePipeline(Item)
}

// PipelineHandlerFunc is an adapter to allow the use of ordinary
// functions as PipelineHandler.
type PipelineHandlerFunc func(Item)

// ServePipeline performs for given Item data.
func (f PipelineHandlerFunc) ServePipeline(v Item) {
	f(v)
}

// Pipeline allows perform value Item passed one PipelineHandler
// to the next PipelineHandler in the chain.
type Pipeline func(PipelineHandler) PipelineHandler
