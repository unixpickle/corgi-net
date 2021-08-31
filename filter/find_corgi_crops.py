import argparse
import json
import os
from typing import List, Tuple

from PIL import Image
import numpy as np
from timm.data import IMAGENET_DEFAULT_MEAN, IMAGENET_DEFAULT_STD
from timm.models import create_model
import torch
import torch.nn.functional as F
from tqdm.auto import tqdm


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--output-path", type=str, default="crops.json")
    parser.add_argument("--classes", type=str, default="263,264")
    parser.add_argument("image_dir", type=str)
    args = parser.parse_args()

    classes = [int(x) for x in args.classes.split(",")]

    print("creating model...")
    dev = torch.device("cuda" if torch.cuda.is_available() else "cpu")
    classifier = create_model("vit_large_patch32_384", pretrained=True).to(dev)

    patch_coords = {}
    if os.path.exists(args.output_path):
        with open(args.output_path, "rt") as f:
            patch_coords = json.load(f)

    def save_coords():
        with open(args.output_path + ".tmp", "wt+") as f:
            json.dump(patch_coords, f)
        os.rename(args.output_path + ".tmp", args.output_path)

    print("scoring crops...")
    idx = 0
    for name in tqdm(sorted(os.listdir(args.image_dir))):
        if name.startswith(".") or not name.endswith(".jpg"):
            continue
        if name.split(".")[0] in patch_coords:
            continue
        img_path = os.path.join(args.image_dir, name)
        pil_img = Image.open(img_path)
        crops, bboxes = crops_of_image(pil_img, dev)
        with torch.no_grad():
            outs = F.softmax(classifier(crops), dim=-1)
            scores = outs[:, classes[0]]
            for c in classes[1:]:
                scores += outs[:, c]
            scores = scores.cpu().numpy().tolist()
        patch_coords[name.split(".")[0]] = dict(scores=scores, bboxes=bboxes)
        if not idx % 100:
            save_coords()
        idx += 1

    save_coords()


def crops_of_image(
    img: Image,
    device: torch.device,
    crop_size: int = 384,
) -> Tuple[torch.Tensor, List[Tuple[float, float, float, float]]]:
    width, height = img.size
    small_side = min(width, height)

    min_scale = crop_size / small_side
    max_scale = 1.0
    scales = sorted(set([min_scale, min(max_scale, min_scale * 1.5)]))

    slices = []
    boxes = []
    for scale in scales:
        resized = img.resize((round(width * scale), round(height * scale)))

        torch_img = (
            (
                (
                    (torch.from_numpy(np.array(resized)).float() / 255)
                    - torch.tensor(IMAGENET_DEFAULT_MEAN)
                )
                / torch.tensor(IMAGENET_DEFAULT_STD)
            )
            .permute(2, 0, 1)
            .contiguous()
            .to(device)
        )

        def coord_splits(size):
            values = set()
            for frac in [0, 0.25, 0.5, 0.75, 1.0]:
                values.add(round((size - crop_size) * frac))
            return sorted(values)

        for y in coord_splits(torch_img.shape[1]):
            for x in coord_splits(torch_img.shape[2]):
                box = (
                    x / scale,
                    y / scale,
                    crop_size / scale,
                    crop_size / scale,
                )
                if box not in boxes:
                    boxes.append(box)
                    slices.append(
                        torch_img[:, y : y + crop_size, x : x + crop_size].contiguous()
                    )

    return torch.stack(slices), boxes


if __name__ == "__main__":
    main()
