import matplotlib
import matplotlib.pyplot as plt
import argparse
import csv
import os
import pandas as pd

matplotlib.rcParams['pdf.fonttype'] = 42
matplotlib.rcParams['ps.fonttype'] = 42
matplotlib.rcParams['text.usetex'] = True

def plot_results(time_data, latency_data, rps_data, output_filename):
    """Generates and saves a dual-axis plot of latency and offered RPS over time."""
    matplotlib.rcParams['pdf.fonttype'] = 42
    matplotlib.rcParams['ps.fonttype'] = 42

    fig, ax1 = plt.subplots(figsize=(12, 6))

    # Plot Latency on the left Y-axis
    color = 'tab:red'
    ax1.set_xlabel('Time (s)')
    ax1.set_ylabel('Avg Latency (ms)', color=color)
    ax1.plot(time_data, latency_data, color=color, label='Avg Latency')
    ax1.tick_params(axis='y', labelcolor=color)
    ax1.grid(True)

    # Create a second Y-axis for the Offered RPS
    ax2 = ax1.twinx()
    color = 'tab:blue'
    ax2.set_ylabel('Offered RPS', color=color)
    ax2.plot(time_data, rps_data, color=color, linestyle='--', label='Offered RPS')
    ax2.tick_params(axis='y', labelcolor=color)

    fig.suptitle('System Latency vs. Offered Load Over Time', fontsize=16)
    fig.tight_layout(rect=[0, 0, 1, 0.96])  # Adjust layout to make room for title

    # Add a single legend for both lines
    lines, labels = ax1.get_legend_handles_labels()
    lines2, labels2 = ax2.get_legend_handles_labels()
    ax2.legend(lines + lines2, labels + labels2, loc='upper left')

    plt.savefig(output_filename)
    print(f"\nPlot saved to {output_filename}")


def main(args):
    files = args.input_files
    output_file = args.output_file if args.output_file else "sigmoid_latency_plot.png"

    plt.style.use('seaborn-v0_8-whitegrid')
    fig, ax1 = plt.subplots(figsize=(4, 1.75))

    # --- Primary Y-axis (Latency) ---
    ax1.set_xlabel('Time (s)', fontsize=8)
    ax1.set_ylabel('Tail Latency (ms)', fontsize=8)
    ax1.set_ylim(0, 25)
    ax1.set_yticks([0, 5, 10, 15, 20, 25])

    # --- Secondary Y-axis (RPS) ---
    ax2 = ax1.twinx()
    ax2.set_ylabel('Input Load (RPS)', fontsize=8, color='tab:purple')
    ax2.tick_params(axis='y', labelcolor='tab:purple')
    ax2.set_ylim(0, 300)
    ax2.set_yticks([0, 60, 120, 180, 240, 300])

    # --- Shared horizontal grid lines (dotted) ---
    for y in ax1.get_yticks():
        ax1.axhline(y, color='gray', linestyle=':', linewidth=0.5, zorder=0)

    # Remove default grid
    ax1.grid(False)
    ax2.grid(False)

    # --- X-axis: Remove padding so data touches left/right edges ---
    ax1.margins(x=0)
    ax2.margins(x=0)
    ax1.set_xlim(0, 60)
    ax1.set_xticks([0, 10, 20, 30, 40, 50, 60])

    rps_plotted = False

    for file_path in files:
        try:
            df = pd.read_csv(file_path)
            label = os.path.splitext(os.path.basename(file_path))[0]
            legend_label = 'Baseline'
            if 'delegate' in label:
                legend_label = 'Monolift'
            elif 'microservice' in label:
                legend_label = 'Microservice'

            # Plot latency for the current file on the primary axis
            ax1.plot(df['Time (s)'], df['Avg Latency (ms)'], marker='', linestyle='-', label=legend_label)

            # Plot the offered RPS on the secondary axis (only once)
            if not rps_plotted:
                ax2.plot(df['Time (s)'], df['Offered RPS'], color='tab:purple', linestyle='--')
                rps_plotted = True
        except Exception as e:
            print(f"Could not process file {file_path}: {e}")
            continue
    fig.subplots_adjust(left=0.1, right=0.9, top=0.9, bottom=0.1)

    # --- Formatting and Legend ---
    lines, labels = ax1.get_legend_handles_labels()
    lines2, labels2 = ax2.get_legend_handles_labels()
    ax1.legend(lines + lines2, labels + labels2, loc='upper left', fontsize=8)
    fig.tight_layout(pad=0, rect=[0, 0, 1, 1])  # Remove all padding and margins
    # Save as PDF
    plt.savefig(output_file.replace('.png', '.pdf'), dpi=600)
    print(f"\nPlot saved to {output_file}")


if __name__ == "__main__":
    parser = argparse.ArgumentParser('Scaling Test Runner')
    parser.add_argument("--input-files", type=str, nargs='+', required=True, help="List of input CSV files containing latency and RPS data")
    parser.add_argument("--output-file", type=str, default=None, help="Optional path to the output png file")
    args = parser.parse_args()

    main(args)
