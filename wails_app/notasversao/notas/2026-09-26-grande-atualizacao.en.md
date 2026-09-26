# Journey Mode, new review activities and simplified sync

## New features
- New Journey Review Mode: interactive map with a skill tree, themed levels, progressive missions, and error recovery at the end.
- New Jigsaw Puzzle activities (Meaning, Phonetics, and Trio): interactive piece matching with dynamic physics and animations.
- Sentence Comprehension activity: questions and contextual interpretation of sentences, with support for intelligent generation via Gemini.
- Sentence Building activity: arrange word blocks to build complete sentences in Chinese.
- Pinyin Queue and Pinyin Word: fast-paced exercises focused on auditory recognition and phonetic reinforcement.
- Keyboard Typing in Reviews: option to type answers using either pinyin or hanzi characters, with smart tone and diacritic tolerance.
- Character Focus: pin priority words and characters for intensive reinforcement during review sessions.
- Activity Settings Panel: customize which exercise types and dynamics are included in your review sessions.
- Full Multilingual Interface: complete support for Portuguese, English, and Spanish, along with tens of thousands of new translated example sentences.
- Simplified Cloud Sync: direct connection to Google Drive via a secure bridge, without needing to create API keys or set up manual credentials.
- Visual Illustrations for Hanzi: new built-in imagery to aid in memorizing essential characters and vocabulary.

## Improvements
- Character Decomposition Family Tree: initial display optimized to the first 3 generations, with buttons to expand remaining generations and branch shortcuts on nodes.
- Optimized Audio Storage: the speech synthesis (TTS) cache now stores individual files on disk, drastically reducing database size and speeding up cloud sync.
- Study Suggestions Pop-up: full-screen overlay with protection against accidental dismissals and clean exit when switching tabs.
- Turtle Audio Playback: slower, spaced word-by-word reading in phonetic activities to train listening perception.
- Voice Pre-caching: phrase and word audio clips are generated in the background to eliminate pauses during exercises.
- Updated Performance Panel: new storage metrics showing file count and disk space used by audio clips.

## Fixes
- Review Error Queue: questions intentionally skipped no longer reappear in the final error recovery round.
- Screen Watcher Stability: improved window exclusion to prevent study overlays from flickering on the screen on Windows.
- Automatic Audio Migration: previously synthesized database audios are automatically migrated to disk files on startup.
- Answer Evaluation: improved accuracy when matching polyphonic pinyin readings and calculating typing similarity.
