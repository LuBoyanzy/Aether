using System;
using System.Globalization;
using LibreHardwareMonitor.Hardware;

class Program
{
  static void Main()
  {
    var computer = new Computer
    {
      IsCpuEnabled = true,
      IsGpuEnabled = true,
      IsMemoryEnabled = true,
      IsMotherboardEnabled = true,
      IsStorageEnabled = true,
    };
    computer.Open();

    var reader = Console.In;
    var writer = Console.Out;

    string line;
    while ((line = reader.ReadLine()) != null)
    {
      if (line.Trim().Equals("getTemps", StringComparison.OrdinalIgnoreCase))
      {
        foreach (var hw in computer.Hardware)
        {
          ProcessSensors(hw, writer);
          foreach (var subhardware in hw.SubHardware)
          {
            ProcessSensors(subhardware, writer);
          }
        }
        writer.WriteLine();
        writer.Flush();
      }
    }
    computer.Close();
  }

  static void ProcessSensors(IHardware hardware, System.IO.TextWriter writer)
  {
    hardware.Update();
    foreach (var sensor in hardware.Sensors)
    {
      if (sensor.SensorType == SensorType.Temperature)
      {
        writer.WriteLine($"{sensor.Name}:{sensor.Value?.ToString(CultureInfo.InvariantCulture)}:{sensor.Identifier}");
      }
    }
  }
}
