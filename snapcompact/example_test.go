package snapcompact

import (
	"context"
	"fmt"
)

func ExampleProject() {
	input := Input{History: []Record{{ID: "message-1", Role: "user", Kind: "prose", Text: "Hello   world"}}, CurrentTask: "  Keep my task unchanged.\n"}
	projection, err := Project(context.Background(), input, DefaultPreprocessConfig())
	if err != nil {
		panic(err)
	}
	fmt.Println(projection.Version)
	fmt.Println(projection.CurrentTask == input.CurrentTask)
	fmt.Println(len(projection.Segments))
	// Output:
	// webmux-snapcompact-v1
	// true
	// 1
}

func ExampleNewRenderer() {
	ctx := context.Background()
	projection, err := Project(ctx, Input{History: []Record{{ID: "example", Role: "user", Text: "Hello 中文"}}}, DefaultPreprocessConfig())
	if err != nil {
		panic(err)
	}
	renderer, err := NewRenderer()
	if err != nil {
		panic(err)
	}
	result, err := renderer.Render(ctx, projection, DefaultRenderConfig())
	if err != nil {
		panic(err)
	}
	fmt.Println(len(result.Pages), result.Pages[0].Width, len(result.Uncovered))
	// Output: 1 1024 0
}
