# LLM usage

To build the initial version of this project, I experimented with an LLM model:
Gemini Pro 2.5 (experimental). This page contains a description of how the
experiment went, and what things I learnt.

## How it went

- At first, I wasn't sure of what my needs were or what I needed to build. So I
  started by explaining Gemini what my initial needs were (building a Cadiretis
  clone for nostalgic reasons, and the fact that I needed to track my working
  locations), and asked it to help me create the project plan you can read at
  [//docs/project_plan](./project_plan/README.md).
- Iteratively, we created the project plan section by section, sometimes
  reviewing previous sections as well. We finally reordered some of the
  information and sections in order to make it clearer for a first-time reader,
  since some concepts appeared before they were explained.
- Once that was done, I asked Gemini to generate the code directly. This
  resulted in a code soup, which I had to heavily refactor afterwards. Although
  the initial commit incorporates this refactor, some bad code was still left
  not refactored due to lack of time.

## Things I've learnt

- Instead of asking the model to directly output code, I will try to instruct
  the model to follow a development cycle. For instance, I might want to
  incorporate TDD into the process.
- Next time I will make sure to ask for unit tests in the technical
  requirements.
- I asked the LLM to use Bazel as the build system, but I wasn't too much
  familiarized with it, so the road was bumpy. I learned it's better to learn
  about a tool before asking a LLM to use it (note for the future: maybe LLMs
  can still help in the learning process?), since the decisions that the LLM
  takes are not sometimes the best ones, and some issues I had would have
  easily been fixed if I had read the documentation beforehand.
- I quickly took over after the initial version, due to some issues with the
  code and the fact that it was painful to instruct the AI to fix the issues. I
  might want to try having a more educational approach with the model in the
  future.
- Related to the previous point: using [Gemini's web UI][gemini] for building a
  codebase was painful. I tried to use the canvas feature, but it was hard to
  sync local changes to the generated Canvas (so sometimes I kept the changes
  locally), and it took a lot of time for the model to regenerate it. Towards
  the end, Gemini started to create a lot of canvases so I ended up with
  duplicate files and a whole mess. Maybe AI agents (see
  [avante.nvim][avante-nvim]) can help alleviate this pain point? It's
  something to research for the future.

[gemini]: https://gemini.google.com/
[avante-nvim]: https://github.com/yetone/avante.nvim
