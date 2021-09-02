import json
import os
from typing import Any, Dict, Optional, Tuple

from PIL import Image
from torch.utils.data import Dataset
from torchvision.datasets.utils import download_and_extract_archive


class CorgiNetDataset(Dataset):
    """
    A dataset of corgi images and corresponding metadata.

    Items in this dataset are tuples of the form (image, crops), where `image`
    is a PIL image, and `crops` is a dict describing which square regions of
    the image are most likely to contain a corgi.

    The crops dicts contain "scores" and "bboxes" keys. The "scores" entry is a
    list of floating point probabilities representing corgi predictions, and
    the "bboxes" entry is a list of (x, y, width, height) crop regions which
    correspond to each score. Note that the coordinates and sizes are floats
    and you will likely want to round them before cropping.

    :param data_dir: the directory containing the dataset files.
    :param split: the split of the dataset to use, ("train" or "test").
    :param download: if True and the split directory does not exist, download
                     it from the internet.
    """

    def __init__(
        self,
        data_dir: str,
        split: str = "train",
        download: bool = True,
    ):
        if split not in ["train", "test"]:
            raise ValueError(f"unknown split: {split}")
        self.data_dir = data_dir
        self.images_dir = os.path.join(self.data_dir, "images")
        self.crops_path = os.path.join(self.data_dir, "crops.json")
        self.split = split

        if not os.path.exists(data_dir):
            os.mkdir(data_dir)

        if not os.path.exists(self.crops_path):
            if not download:
                raise FileNotFoundError(f"crop metadata not found: {self.crops_path}")
            data_url = f"https://data.aqnichol.com/corgi-net/crops.json.gz"
            download_and_extract_archive(
                data_url,
                self.data_dir,
                self.data_dir,
                filename=f"crops.json.gz",
                remove_finished=True,
            )

        if not os.path.exists(self.images_dir):
            if not download:
                raise FileNotFoundError(f"image directory not found: {self.images_dir}")
            data_url = f"https://data.aqnichol.com/corgi-net/images.tar"
            download_and_extract_archive(
                data_url,
                self.data_dir,
                self.images_dir,
                filename=f"images.tar",
                remove_finished=True,
            )

        with open(self.crops_path, "rt") as f:
            self.crops = json.load(f)

        self.image_hashes = sorted(self.crops.keys())
        if split == "train":
            self.image_hashes = self.image_hashes[1000:]
        else:
            self.image_hashes = self.image_hashes[:1000]

    def __len__(self) -> int:
        return len(self.image_hashes)

    def __getitem__(self, idx: int) -> Tuple[Any, Dict[str, Any]]:
        hash = self.image_hashes[idx]
        img = Image.open(os.path.join(self.images_dir, f"{hash}.jpg"))
        crop_info = self.crops[hash].copy()
        crop_info["bboxes"] = [tuple(x) for x in crop_info["bboxes"]]
        return img, self.crops[hash]


class CroppedCorgiNetDataset(Dataset):
    """
    This dataset is similar to CorgiNetDataset, but it automatically crops the
    images and filters out crops with low corgi probabilities.

    The items in this dataset are simply PIL images, with no provided crop
    information. Multiple items in the dataset may correspond to the same
    image, but capture different crops of it.

    :param data_dir: the directory containing the dataset files.
    :param split: the split of the dataset to use, ("train" or "test").
    :param download: if True and the split directory does not exist, download
                     it from the internet.
    :param min_prob: the minimum corgi probability for a crop to be allowed to
                     enter the dataset.
    :param transform: if specified, a function to apply to each cropped image.
    """

    def __init__(
        self,
        data_dir: str,
        split: str = "train",
        download: bool = True,
        min_prob: float = 0.05,
        transform: Optional[Any] = None,
    ):
        super().__init__()

        self.base_dataset = CorgiNetDataset(
            data_dir=data_dir,
            split=split,
            download=download,
        )
        self.min_prob = min_prob
        self.transform = transform

        self.crop_pairs = []
        for i, hash in enumerate(self.base_dataset.image_hashes):
            crop_info = self.base_dataset.crops[hash]
            for score, bbox in zip(crop_info["scores"], crop_info["bboxes"]):
                if score >= min_prob:
                    self.crop_pairs.append((i, tuple(int(x) for x in bbox)))
        self._used_images = len(set(i for i, _ in self.crop_pairs))

    def used_images(self) -> int:
        """
        Get the number of unique images for which some crop satisfied the
        minimum corgi probability threshold.
        """
        return self._used_images

    def __len__(self) -> int:
        return len(self.crop_pairs)

    def __getitem__(self, idx: int) -> Any:
        base_index, (x, y, w, h) = self.crop_pairs[idx]
        base_image, _ = self.base_dataset[base_index]
        out_image = base_image.crop(box=(x, y, x + w, y + h))
        if self.transform:
            out_image = self.transform(out_image)
        return out_image
