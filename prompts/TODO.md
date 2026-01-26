1. classifier:
   A classifier that classifies the user role into one of the following three categories (no single use case is independent from each other, and all sub-agents, should be able to collaborate with each other) need a robust prompt for each:
   1. infrastructure engineer
   2. application developer
   3. network engineer

   monitor_types:
   apm
   dbm
   logs
   synthetics

2. RLM (infiinte context) -> synergies with rag pipeline:
   RLM with REPL. We implement an RLM that loads its context as a string in the memory of a Python REPL environment. The REPL environment also loads in a module that allows it to query a sub-LM inside the environment. The system prompt is fixed across all experiments (see Ap- pendix D). For the GPT-5 experiments, we use GPT-5-mini for the recursive LMs and GPT-5 for the root LM, as we found this choice to strike a powerful tradeoff between the capabilities of RLMs and the cost of the recursive calls.
