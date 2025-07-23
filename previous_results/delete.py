import os

# === CONFIG ===
base_dir = "./balanced_results_mixed"  # Top-level directory containing subdirectories
match_string = "mixed-throughput-1.csv"                 # Files must include this substring to be kept

# === SCRIPT ===
for root, dirs, files in os.walk(base_dir):
    for file in files:
        if match_string not in file:
            file_path = os.path.join(root, file)
            print(f"Deleting: {file_path}")
            os.remove(file_path)