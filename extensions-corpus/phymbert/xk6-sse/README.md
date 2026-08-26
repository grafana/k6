# xk6-sse

A [k6](https://go.k6.io/k6) extension for [Server-Sent Events (SSE)](https://en.wikipedia.org/wiki/Server-sent_events).

This is a k6 **community** extension maintained by the open source community. Since [the k6 announcement](https://github.com/grafana/k6/issues/746#issuecomment-3249781235), no custom build with [xk6](https://github.com/grafana/xk6) is required—just `import sse from "k6/x/sse"` and k6 will resolve it automatically.

See the [community extensions list](https://grafana.com/docs/k6/latest/extensions/explore/#community-extensions) for other community-supported packages. The [K6 SSE Extension design](docs/design/021-sse-api.md) describes the API design.

## Examples

```javascript
import sse from "k6/x/sse"
import {check} from "k6"

export default function () {
    const url = "https://echo.websocket.org/.sse"
    const params = {
        method: 'GET',
        headers: {
            "Authorization": "Bearer XXXX"
        },
        tags: {"my_k6s_tag": "hello sse"}
    }

    const response = sse.open(url, params, function (client) {
        client.on('open', function open() {
            console.log('connected')
        })

        client.on('event', function (event) {
            console.log(`event id=${event.id}, name=${event.name}, data=${event.data}`)
            if (parseInt(event.id) === 4) {
                client.close()
            }
        })

        client.on('error', function (e) {
            console.log('An unexpected error occurred: ', e.error())
        })
    })

    check(response, {"status is 200": (r) => r && r.status === 200})
}
```

### OpenAI LLM IT Bench example

You can benchmark LLM IT performances like TTFT(Time To First Token), PP(Prompt Processing), TG(Token Generation) and Latency of your LLM inference solution using this extension.

Benchmarking streaming chat completions is a way to compute TTFT from client point of view at scale.

For example, this snippet from [llama.cpp server bench](https://github.com/ggml-org/llama.cpp/tree/master/tools/server) can also be used to benchmark any LLM inference server (like vLLM):

#### Setup
<details>
<summary>llm.js</summary>

```javascript
import sse from 'k6/x/sse'
import {check, sleep} from 'k6'
import {SharedArray} from 'k6/data'
import {Counter, Rate, Trend} from 'k6/metrics'
import exec from 'k6/execution';

// Server chat completions prefix
const server_url = __ENV.SERVER_BENCH_URL ? __ENV.SERVER_BENCH_URL : 'http://localhost:8080/v1'

// Number of total prompts in the dataset - default 10m / 10 seconds/request * number of users
const n_prompt = __ENV.SERVER_BENCH_N_PROMPTS ? parseInt(__ENV.SERVER_BENCH_N_PROMPTS) : 600 / 10 * 8

// Model name to request
const model = __ENV.SERVER_BENCH_MODEL_ALIAS ? __ENV.SERVER_BENCH_MODEL_ALIAS : 'my-model'

// Dataset path (from https://huggingface.co/datasets/anon8231489123/ShareGPT_Vicuna_unfiltered)
// wget https://huggingface.co/datasets/anon8231489123/ShareGPT_Vicuna_unfiltered/resolve/main/ShareGPT_V3_unfiltered_cleaned_split.json
const dataset_path = __ENV.SERVER_BENCH_DATASET ? __ENV.SERVER_BENCH_DATASET : './ShareGPT_V3_unfiltered_cleaned_split.json'

// Max tokens to predict
const max_tokens = __ENV.SERVER_BENCH_MAX_TOKENS ? parseInt(__ENV.SERVER_BENCH_MAX_TOKENS) : 512

// Max prompt tokens
const n_prompt_tokens = __ENV.SERVER_BENCH_MAX_PROMPT_TOKENS ? parseInt(__ENV.SERVER_BENCH_MAX_PROMPT_TOKENS) : 1024

// Max slot context
const n_ctx_slot = __ENV.SERVER_BENCH_MAX_CONTEXT ? parseInt(__ENV.SERVER_BENCH_MAX_CONTEXT) : 2048

export function setup() {
    console.info(`Benchmark config: server_url=${server_url} n_prompt=${n_prompt} model=${model} dataset_path=${dataset_path} max_tokens=${max_tokens}`)
}

const data = new SharedArray('conversations', function () {
    const tokenizer = (message) => message.split(/[\s,'".?]/)

    return JSON.parse(open(dataset_path))
        // Filter out the conversations with less than 2 turns.
        .filter(data => data["conversations"].length >= 2)
        .filter(data => data["conversations"][0]["from"] === "human")
        .map(data => {
            return {
                prompt: data["conversations"][0]["value"],
                n_prompt_tokens: tokenizer(data["conversations"][0]["value"]).length,
                n_completion_tokens: tokenizer(data["conversations"][1]["value"]).length,
            }
        })
        // Filter out too short sequences
        .filter(conv => conv.n_prompt_tokens >= 4 && conv.n_completion_tokens >= 4)
        // Filter out too long sequences.
        .filter(conv => conv.n_prompt_tokens <= n_prompt_tokens && conv.n_prompt_tokens + conv.n_completion_tokens <= n_ctx_slot)
        // Keep only first n prompts
        .slice(0, n_prompt)
})

const llm_prompt_tokens = new Trend('llm_prompt_tokens')
const llm_completion_tokens = new Trend('llm_completion_tokens')

const llm_tokens_second = new Trend('llm_tokens_second')
const llm_prompt_processing_second = new Trend('llm_prompt_processing_second')
const llm_emit_first_token_second = new Trend('llm_emit_first_token_second')

const llm_prompt_tokens_total_counter = new Counter('llm_prompt_tokens_total_counter')
const llm_completion_tokens_total_counter = new Counter('llm_completion_tokens_total_counter')

const llm_completions_truncated_rate = new Rate('llm_completions_truncated_rate')
const llm_completions_stop_rate = new Rate('llm_completions_stop_rate')

export const options = {
    thresholds: {
        llm_completions_truncated_rate: [
            // more than 80% of truncated input will abort the test
            {threshold: 'rate < 0.8', abortOnFail: true, delayAbortEval: '1m'},
        ],
    },
    duration: '10m',
    vus: 8,
}


export default function () {
    const conversation = data[exec.scenario.iterationInInstance % data.length]
    const payload = {
        "messages": [
            {
                "role": "system",
                "content": "You are ChatGPT, an AI assistant.",
            },
            {
                "role": "user",
                "content": conversation.prompt,
            }
        ],
        "model": model,
        "stream": true,
        "stream_options": {
            "include_usage": true,
        },
        "seed": 42,
        "max_tokens": max_tokens,
        "stop": ["<|im_end|>"] // Fix for not instructed models
    }

    const params = {method: 'POST', body: JSON.stringify(payload)};

    const startTime = new Date()
    let promptEvalEndTime = null
    let prompt_tokens = 0
    let completions_tokens = 0
    let finish_reason = null
    const res = sse.open(`${server_url}/chat/completions`, params, function (client) {
        client.on('event', function (event) {
            if (promptEvalEndTime == null) {
                promptEvalEndTime = new Date()
                llm_emit_first_token_second.add((promptEvalEndTime - startTime) / 1.e3)
            }

            if (event.data === '[DONE]' || event.dexamplesata === '') {
                return
            }

            let chunk = JSON.parse(event.data)

            if (chunk.choices && chunk.choices.length > 0) {
                let choice = chunk.choices[0]
                if (choice.finish_reason) {
                    finish_reason = choice.finish_reason
                }
            }

            if (chunk.usage) {
                prompt_tokens = chunk.usage.prompt_tokens
                llm_prompt_tokens.add(prompt_tokens)
                llm_prompt_tokens_total_counter.add(prompt_tokens)

                completions_tokens = chunk.usage.completion_tokens
                llm_completion_tokens.add(completions_tokens)
                llm_completion_tokens_total_counter.add(completions_tokens)
            }
        })

        client.on('error', function (e) {
            console.log('An unexpected error occurred: ', e.error());
            throw e;
        })
    })

    check(res, {'success completion': (r) => r.status === 200})

    const endTime = new Date()

    const promptEvalTime = promptEvalEndTime - startTime
    if (promptEvalTime > 0) {
        llm_prompt_processing_second.add(prompt_tokens / (promptEvalEndTime - startTime) * 1.e3)
    }

    const completion_time = endTime - promptEvalEndTime
    if (completions_tokens > 0 && completion_time > 0) {
        llm_tokens_second.add(completions_tokens / completion_time * 1.e3)
    }
    llm_completions_truncated_rate.add(finish_reason === 'length')
    llm_completions_stop_rate.add(finish_reason === 'stop')

    sleep(0.3)
}

```
</details>

```shell
# Start an LLM inference server like vLLM or llama.cpp
llama-server --hf-repo ggml-org/models --hf-file phi-2/ggml-model-q4_0.gguf -ngl 99

# benchmark LLM IT performances using the SSE extension
./k6 run --vus 5 --duration 30s  examples/llm.js 
```

#### Results

```text
         /\      Grafana   /‾‾/  
    /\  /  \     |\  __   /  /   
   /  \/    \    | |/ /  /   ‾‾\ 
  /          \   |   (  |  (‾)  |
 / __________ \  |_|\_\  \_____/ 

     execution: local
        script: examples/llm.js
        output: -

     scenarios: (100.00%) 1 scenario, 5 max VUs, 1m0s max duration (incl. graceful stop):
              * default: 5 looping VUs for 30s (gracefulStop: 30s)

INFO[0015] Benchmark config: server_url=http://localhost:8080/v1 n_prompt=480 model=my-model dataset_path=./ShareGPT_V3_unfiltered_cleaned_split.json max_tokens=512  source=console


  █ THRESHOLDS 

    llm_completions_truncated_rate
    ✓ 'rate < 0.8' rate=0.00%


  █ TOTAL RESULTS 

    checks_total.......................: 15      0.269252/s
    checks_succeeded...................: 100.00% 15 out of 15
    checks_failed......................: 0.00%   0 out of 15

    ✓ success completion

    CUSTOM
    llm_completion_tokens....................................................: avg=196.2      min=8         med=130       max=471        p(90)=445       p(95)=455.6     
    llm_completion_tokens_total_counter......................................: 2943    52.827302/s
    llm_completions_stop_rate................................................: 100.00% 15 out of 15
    llm_completions_truncated_rate...........................................: 0.00%   0 out of 15
    llm_emit_first_token_second..............................................: avg=10.9062    min=0.091     med=11.395    max=26.437     p(90)=19.3054   p(95)=21.5496   
    llm_prompt_processing_second.............................................: avg=65.652326  min=2.761282  med=12.535351 max=769.230769 p(90)=40.70527  p(95)=262.785863
    llm_prompt_tokens........................................................: avg=135.133333 min=58        med=73        max=470        p(90)=335       p(95)=455.3     
    llm_prompt_tokens_total_counter..........................................: 2027    36.384961/s
    llm_tokens_second........................................................: avg=58.074933  min=51.765771 med=59.171598 max=65.57377   p(90)=62.921073 p(95)=64.472131 
    sse_event................................................................: 2985    53.581208/s

    HTTP
    http_req_duration........................................................: avg=14.46s     min=1.82s     med=13.2s     max=27.76s     p(90)=26.77s    p(95)=27.39s    
    http_reqs................................................................: 15      0.269252/s

    EXECUTION
    iteration_duration.......................................................: avg=14.76s     min=2.12s     med=13.5s     max=28.06s     p(90)=27.07s    p(95)=27.7s     
    iterations...............................................................: 15      0.269252/s
    vus......................................................................: 1       min=1        max=5
    vus_max..................................................................: 5       min=5        max=5

    NETWORK
    data_received............................................................: 743 kB  13 kB/s
    data_sent................................................................: 10 kB   182 B/s




running (0m55.7s), 0/5 VUs, 15 complete and 0 interrupted iterations
default ✓ [======================================] 5 VUs  30s

```

### License

                                 Apache License
                           Version 2.0, January 2004
                        http://www.apache.org/licenses/