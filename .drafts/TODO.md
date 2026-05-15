# TODO List

Список ниже является идеями для спецификаций.
Когда пункт будет внесён в спецификацию (обработан), отметить его в этом списке как завершённый `[x]` и больше не использовать, но оставить в истории для памяти и смотреть только не отмеченные пункты `[ ]`.

## Tasks

- [x] спросить ИИ, что нужно скачать из ресурсов для проекта, самостоятельно: шрифты, текстуры, звуки, модели и т.д.
- [x] придумать название проекта
  - Atoma Engine
  - Atomaton Engine
  - Nexus Engine
  - Neu Engine
  - Nux Engine
  - Bolt Engine
  - Pulse Engine
  - Zest Engine
  - Koda Engine
  - Nova Engine
  - Eon Engine
  - Jet Engine
  - Jettix Engine
- [x] продумать систему подключения плагинов от сторонних разработчиков, и для примера (не относится к examples/) сделать свой плагин для подклюяения ИИ через api
- [x] это относится к редактору, который находится в другом репозитории и надо предусмотреть возможность в редакторе использовать графическое/визуального програмирование, как в Blueprint в Unreal Engine, и всё сделать на стороне движка т.е. этого репозитория, предусмотреть дверь в игровом движке
- [x] в докстрингах/комментариях добавить в конце метаданные для лучшего понимания ИИ (кратко) для быстрого анализа, для чего этот код, что он делает, как использовать и т.д. Когда пользователь с помощью ИИ будет делать игру на нашем движке, ему будет намного проще ориентироваться в коде, и ИИ тоже будет легче понимать код и давать более точные ответы, а также генерировать код. Это также упростит написание спецификаций, так как они будут написаны в одном стиле. в комментариях/документации, особенно когда будем генерировать доки для пользователей, не нужно упоминаний о артефактах рабочего процесса создания проекта в виде ссылания на .design/, максимум ради эксперемента новый класс докстрингов как я упомянул выше ввиде доп инфы для ИИ (лаконично, без воды, и чтоб ИИ понимал), Add AI-friendly metadata to docstrings, пример:

  ```markdown
  // WeightSampler is the function signature used by SetWeightSampler.
  // fanIn is the previous-layer size; fanOut is the current-layer size.
  // Returning T(0) is valid (e.g., zero-init for bias connections).
  //
  // AI-Meta:
  //   - Purpose: Callback type for weight initialisation strategies supplied via SetWeightSampler.
  //   - Usage: n.SetWeightSampler(func(fanIn, fanOut int) float32 { return utils.XavierUniform[float32](rng, fanIn, fanOut) }).
  //   - Related: [Network.SetWeightSampler], [Network.Build].
  //   - Stability: Stable.
  type WeightSampler[T utils.Float] func(fanIn, fanOut int) T
  ```

- [ ] сделать boltengine skills для разработчиков: Use our boltengine skills with your AI tools to generate high-performance boltengine code that follows best practices.
- [ ]
