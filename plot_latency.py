import matplotlib
import pandas as pd
import matplotlib.pyplot as plt
import os
import argparse
import seaborn as sns

matplotlib.rcParams['pdf.fonttype'] = 42
matplotlib.rcParams['ps.fonttype'] = 42

def plot_latency_data(directory=".", metric="avg", outfile="throughput_latency.pdf"):
    """
    Plots throughput vs. latency from CSV files in a given directory.
    """
    plt.style.use('seaborn-v0_8-whitegrid')
    fig, ax = plt.subplots(figsize=(4, 2))

    x_col = "Actual RPS"
    y_col = "p95 Latency (ms)" if metric == "p95" else "Avg Latency (ms)"


    plot_order = [
        'baseline',
        'full',
        'post-only',
        'user-only',
        'SG-only',
        'TL-only',
    ]

    # Reverse the plot order for plotting, but keep legend order as original
    plot_order_reversed = list(reversed(plot_order))

    # Assign colors so legend matches original order
    palette = sns.color_palette("tab10", len(plot_order))
    plot_colors = {name: palette[i] for i, name in enumerate(plot_order)}

    # Plot in reversed order
    # Define a list of distinct markers for each configuration
    markers = ['o', 's', 'D', '^', 'v', 'P']
    marker_map = {name: markers[i % len(markers)] for i, name in enumerate(plot_order)}

    for fname in plot_order_reversed:
        filename = f"{fname}.csv"
        if filename.endswith(".csv"):
            label = os.path.splitext(filename)[0]
            filepath = os.path.join(directory, filename)

            try:
                df = pd.read_csv(filepath)
                if all(col in df.columns for col in [x_col, y_col]):
                    ax.plot(
                        df[x_col],
                        df[y_col],
                        marker=marker_map.get(label, 'o'),
                        linestyle='-',
                        label=label,
                        color=plot_colors.get(label, 'black'),
                        markersize=5,
                        linewidth=1.2
                    )
                else:
                    print(f"Skipping {filename}: missing required columns.")
            except Exception as e:
                print(f"Could not process {filename}: {e}")

    # Reorder legend to match original plot_order
    handles, labels = ax.get_legend_handles_labels()
    legend_order = [labels.index(name) for name in plot_order if name in labels]
    ax.legend([handles[i] for i in legend_order], [labels[i] for i in legend_order], fontsize=8)
    # palette = sns.color_palette("tab10", len(plot_order))
    # plot_colors = {name: palette[i] for i, name in enumerate(reversed(plot_order))}

    # # --- Plotting P95 and Average Latency ---
    # # for filename in sorted(os.listdir(directory)):
    # for fname in plot_order:
    #     filename = f"{fname}.csv"
    #     if filename.endswith(".csv"):
    #         # Use the filename (without extension) as the label
    #         label = os.path.splitext(filename)[0]
    #         filepath = os.path.join(directory, filename)

    #         try:
    #             df = pd.read_csv(filepath)
    #             # Ensure required columns exist
    #             if all(col in df.columns for col in [x_col, y_col]):
    #                 # Plot P95 latency as a solid line with markers
    #                 ax.plot(df[x_col], df[y_col], marker='.', linestyle='-', label=label, color=plot_colors.get(label, 'black'))
    #             else:
    #                 print(f"Skipping {filename}: missing required columns.")
    #         except Exception as e:
    #             print(f"Could not process {filename}: {e}")

    # --- Formatting the Plot ---
    # ax.set_title('Throughput vs. Latency', fontsize=)
    ax.set_xlabel('Throughput (RPS)', fontsize=8)
    ax.set_ylabel(y_col, fontsize=8)
    # ax.legend(title="Configuration", fontsize=8)
    ax.grid(True, which='both', linestyle='--', linewidth=0.2)

    # Set the y-axis to a log scale with a limit of 500ms, starting at 40ms
    # ax.set_yscale('log')
    ax.set_ylim(0, 100)

    ax.set_xscale('log')
    ax.set_xlim(0, 2000)
    # Set x-axis ticks to powers of 2, starting from 100
    ax.set_xticks([10, 100, 1000])
    ax.set_xticklabels(['$10^1$', '$10^2$', '$10^3$'])

    fig.tight_layout(pad=0)


    # --- Save and Show ---
    plt.savefig(outfile, dpi=600, bbox_inches='tight', pad_inches=0)
    print(f"Plot saved to {outfile}")
    plt.show()

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Plot throughput vs. latency from CSV files.")
    parser.add_argument(
        "--directory",
        type=str,
        default=".",
        help="Directory containing CSV files to plot (default: current directory)"
    )
    parser.add_argument("--metric", type=str,default="avg",help="p95 or avg")
    parser.add_argument("--outfile", type=str,default="throughput_latency.pdf")
    args = parser.parse_args()
    # The script will look for CSV files in the directory where it is run.
    plot_latency_data(args.directory, args.metric, args.outfile)
