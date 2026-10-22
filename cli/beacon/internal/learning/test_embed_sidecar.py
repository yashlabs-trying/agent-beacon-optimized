"""Device selection checks without downloading or loading a model."""

import runpy
import sys
import types
import unittest
from pathlib import Path
from unittest.mock import patch


class DeviceSelectionTest(unittest.TestCase):
    def test_selects_available_accelerator(self):
        sidecar = Path(__file__).with_name("embed_sidecar.py")
        for cuda, mps, expected in (
            (True, True, "cuda"),
            (False, True, "mps"),
            (False, False, "cpu"),
        ):
            with self.subTest(cuda=cuda, mps=mps):
                torch = types.ModuleType("torch")
                torch.cuda = types.SimpleNamespace(is_available=lambda: cuda)
                torch.backends = types.SimpleNamespace(
                    mps=types.SimpleNamespace(is_available=lambda: mps)
                )
                transformers = types.ModuleType("sentence_transformers")
                transformers.SentenceTransformer = lambda *args, **kwargs: types.SimpleNamespace(
                    get_sentence_embedding_dimension=lambda: 384
                )
                with patch.dict(
                    sys.modules,
                    {"torch": torch, "sentence_transformers": transformers},
                ):
                    result = runpy.run_path(str(sidecar))
                self.assertEqual(result["device"], expected)


if __name__ == "__main__":
    unittest.main()
